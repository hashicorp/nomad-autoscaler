// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	// pluginName is the name of the plugin
	pluginName = "prometheus"

	// configKeyAddress is the accepted configuration key which holds the
	// address param.
	configKeyAddress = "address"

	// configKeyBasicAuthUser and configKeyBasicAuthPassword are the
	// configuration keys used to set the Prometheus client basic auth.
	configKeyBasicAuthUser     = "basic_auth_user"
	configKeyBasicAuthPassword = "basic_auth_password"

	// configKeyHeadersPrefix is the prefix used to indicate that a
	// configuration value should be set as an HTTP header.
	configKeyHeadersPrefix = "header_"

	// configKeyCACert is the path to the CA certificate the Prometheus client
	// should use.
	configKeyCACert = "ca_cert"

	// configKeySkipVerify indicates that the Prometheus client should not
	// verify TLS certificates.
	configKeySkipVerify = "skip_verify"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewPrometheusPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}
)

type APMPlugin struct {
	client api.Client
	config map[string]string
	logger hclog.Logger
}

func NewPrometheusPlugin(log hclog.Logger) apm.APM {
	return &APMPlugin{
		logger: log,
	}
}

func (a *APMPlugin) SetConfig(config map[string]string) error {

	a.config = config

	// If the address is not set, or is empty within the config, any client
	// calls will fail. It seems logical to catch this here rather than just
	// let queries fail.
	addr, ok := a.config[configKeyAddress]
	if !ok || addr == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyAddress)
	}

	tlsConfig, err := generateTLSConfig(config)
	if err != nil {
		return fmt.Errorf("failed to parse TLS configuration: %v", err)
	}

	promCfg := api.Config{
		Address:      addr,
		RoundTripper: newPluginRoudTripper(a.config, tlsConfig),
	}

	// create Prometheus client
	client, err := api.NewClient(promCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Prometheus client: %v", err)
	}

	// store config and client in plugin instance
	a.client = client

	return nil
}

func (a *APMPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

func (a *APMPlugin) Query(q string, r sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	m, err := a.QueryMultiple(q, r)
	if err != nil {
		return nil, err
	}

	switch len(m) {
	case 0:
		return sdk.TimestampedMetrics{}, nil
	case 1:
		return m[0], nil
	default:
		return nil, fmt.Errorf("query returned %d metric streams, only 1 is expected", len(m))
	}
}

func (a *APMPlugin) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	a.logger.Debug("querying Prometheus", "query", q, "range", r)

	v1api := v1.NewAPI(a.client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	promRange := v1.Range{Start: r.From, End: r.To, Step: time.Second}
	result, warnings, err := v1api.QueryRange(ctx, q, promRange)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %v", err)
	}

	// If Prometheus returned warnings, report these to the user.
	for _, w := range warnings {
		a.logger.Warn("prometheus query returned warning", "warning", w)
	}

	switch t := result.Type(); t {
	case model.ValScalar:
		resultScalar := result.(*model.Scalar)
		return parseScalar(resultScalar)
	case model.ValVector:
		resultVector := result.(model.Vector)
		return parseVector(resultVector)
	case model.ValMatrix:
		resultMatrix := result.(model.Matrix)
		return parseMatrix(resultMatrix)
	default:
		return nil, fmt.Errorf("result type (`%v`) is not supported", t)
	}
}

func generateTLSConfig(config map[string]string) (*tls.Config, error) {
	tlsConfig := tls.Config{}

	// Load the CA certificate if present.
	caCertPath := config[configKeyCACert]
	if caCertPath != "" {
		caCert, err := ioutil.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate %s: %v", caCertPath, err)
		}

		block, _ := pem.Decode(caCert)
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM file %s", caCertPath)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	skipVerify := config[configKeySkipVerify]
	if skipVerify != "" {
		skipVerifyBool, err := strconv.ParseBool(skipVerify)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s value %s: %v", configKeySkipVerify, skipVerify, err)
		}

		tlsConfig.InsecureSkipVerify = skipVerifyBool
	}

	return &tlsConfig, nil
}

func parseScalar(s *model.Scalar) ([]sdk.TimestampedMetrics, error) {
	if s == nil {
		return nil, nil
	}

	tm, err := parseSample(*s)
	if err != nil {
		return nil, err
	}

	return []sdk.TimestampedMetrics{{tm}}, nil
}

func parseVector(v model.Vector) ([]sdk.TimestampedMetrics, error) {
	var result sdk.TimestampedMetrics
	for _, s := range v {
		tm, err := parseSample(*s)
		if err != nil {
			return nil, err
		}

		result = append(result, tm)
	}

	return []sdk.TimestampedMetrics{result}, nil
}

func parseMatrix(m model.Matrix) ([]sdk.TimestampedMetrics, error) {
	// Cast matrix to a list of sample streams so we can iterate over it.
	ssList := []*model.SampleStream(m)
	result := make([]sdk.TimestampedMetrics, len(ssList))
	for i, ss := range ssList {
		var metrics sdk.TimestampedMetrics
		for _, sp := range ss.Values {
			tm, err := parseSample(sp)
			if err != nil {
				return nil, err
			}

			metrics = append(metrics, tm)
		}

		result[i] = metrics
	}

	return result, nil
}

func parseSample(s interface{}) (sdk.TimestampedMetric, error) {
	var ts model.Time
	var val model.SampleValue
	var result sdk.TimestampedMetric

	switch sType := s.(type) {
	case model.Scalar:
		val = sType.Value
		ts = sType.Timestamp
	case model.Sample:
		val = sType.Value
		ts = sType.Timestamp
	case model.SamplePair:
		val = sType.Value
		ts = sType.Timestamp
	default:
		return result, fmt.Errorf("invalid sample type %T", s)
	}

	valFloat := float64(val)
	// Check whether the sample value is an IEEE 754 not-a-number value.
	if math.IsNaN(valFloat) {
		return result, errors.New("query result value is not-a-number")
	}

	tsTime := time.Unix(int64(ts)/1e3, 0)

	return sdk.TimestampedMetric{
		Timestamp: tsTime,
		Value:     valFloat,
	}, nil
}
