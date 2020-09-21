package plugin

import (
	"context"
	"fmt"
	"math"
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
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewPrometheusPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
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
	if a.config[configKeyAddress] == "" {
		return fmt.Errorf("%q config value cannot be empty", configKeyAddress)
	}

	promCfg := api.Config{
		Address: a.config[configKeyAddress],
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

func parseScalar(s *model.Scalar) (sdk.TimestampedMetrics, error) {
	if s == nil {
		return nil, nil
	}

	tm, err := parseSample(*s)
	if err != nil {
		return nil, err
	}

	return sdk.TimestampedMetrics{tm}, nil
}

func parseVector(v model.Vector) (sdk.TimestampedMetrics, error) {
	var result sdk.TimestampedMetrics
	for _, s := range v {
		tm, err := parseSample(*s)
		if err != nil {
			return nil, err
		}

		result = append(result, tm)
	}

	return result, nil
}

func parseMatrix(m model.Matrix) (sdk.TimestampedMetrics, error) {
	if m.Len() != 1 {
		return nil, fmt.Errorf("query returned %d metric streams, only 1 is expected.", m.Len())
	}

	// Cast matrix to a list of sample streams so we can get the first stream.
	ssList := []*model.SampleStream(m)
	ss := ssList[0]
	if ss == nil {
		return nil, nil
	}

	var result sdk.TimestampedMetrics
	for _, sp := range ss.Values {
		tm, err := parseSample(sp)
		if err != nil {
			return nil, err
		}

		result = append(result, tm)
	}

	return result, nil
}

func parseSample(s interface{}) (sdk.TimestampedMetric, error) {
	var ts model.Time
	var val model.SampleValue
	var result sdk.TimestampedMetric

	switch s.(type) {
	case model.Scalar:
		val = s.(model.Scalar).Value
		ts = s.(model.Scalar).Timestamp
	case model.Sample:
		val = s.(model.Sample).Value
		ts = s.(model.Sample).Timestamp
	case model.SamplePair:
		val = s.(model.SamplePair).Value
		ts = s.(model.SamplePair).Timestamp
	default:
		return result, fmt.Errorf("invalid sample type %T", s)
	}

	valFloat := float64(val)
	// Check whether the sample value is an IEEE 754 not-a-number value.
	if math.IsNaN(valFloat) {
		return result, fmt.Errorf("query result value is not-a-number")
	}

	tsTime := time.Unix(int64(ts), 0)

	return sdk.TimestampedMetric{
		Timestamp: tsTime,
		Value:     valFloat,
	}, nil
}
