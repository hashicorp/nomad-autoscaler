// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/circonus"
	"github.com/armon/go-metrics/datadog"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/nomad-autoscaler/agent/config"
)

// setupTelemetry is used to setup the telemetry sub-systems and returns the
// in-memory sink to be used in http configuration.
func (a *Agent) setupTelemetry(cfg *config.Telemetry) (*metrics.InmemSink, error) {

	// Setup telemetry using an aggregate of 10 second intervals for 1 minute.
	// Expose the metrics over stderr when there is a SIGUSR1 received.
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metrics.DefaultInmemSignal(inm)

	var telConfig *config.Telemetry
	if cfg == nil {
		telConfig = &config.Telemetry{}
	} else {
		telConfig = cfg
	}

	metricsConf := metrics.DefaultConfig("nomad-autoscaler")
	metricsConf.EnableHostname = !telConfig.DisableHostname
	metricsConf.EnableHostnameLabel = telConfig.EnableHostnameLabel

	// Configure the statsite sink.
	var fanout metrics.FanoutSink
	if telConfig.StatsiteAddr != "" {
		sink, err := metrics.NewStatsiteSink(telConfig.StatsiteAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to setup statsite sink: %v", err)
		}
		fanout = append(fanout, sink)
	}

	// Configure the statsd sink.
	if telConfig.StatsdAddr != "" {
		sink, err := metrics.NewStatsdSink(telConfig.StatsdAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to setup statsd sink: %v", err)
		}
		fanout = append(fanout, sink)
	}

	// Configure the Prometheus sink.
	if telConfig.PrometheusMetrics || telConfig.PrometheusRetentionTime != 0 {
		prometheusOpts := prometheus.PrometheusOpts{
			Expiration: telConfig.PrometheusRetentionTime,
		}

		sink, err := prometheus.NewPrometheusSinkFrom(prometheusOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to setup Promtheus sink: %v", err)
		}
		fanout = append(fanout, sink)
	}

	// Configure the Datadog sink.
	if telConfig.DogStatsDAddr != "" {
		var tags []string

		if telConfig.DogStatsDTags != nil {
			tags = telConfig.DogStatsDTags
		}

		sink, err := datadog.NewDogStatsdSink(telConfig.DogStatsDAddr, metricsConf.HostName)
		if err != nil {
			return nil, fmt.Errorf("failed to setup DogStatsD sink: %v", err)
		}
		sink.SetTags(tags)
		fanout = append(fanout, sink)
	}

	// Configure the Circonus sink.
	if telConfig.CirconusAPIToken != "" || telConfig.CirconusCheckSubmissionURL != "" {
		circonusCfg := &circonus.Config{}
		circonusCfg.Interval = telConfig.CirconusSubmissionInterval
		circonusCfg.CheckManager.API.TokenKey = telConfig.CirconusAPIToken
		circonusCfg.CheckManager.API.TokenApp = telConfig.CirconusAPIApp
		circonusCfg.CheckManager.API.URL = telConfig.CirconusAPIURL
		circonusCfg.CheckManager.Check.SubmissionURL = telConfig.CirconusCheckSubmissionURL
		circonusCfg.CheckManager.Check.ID = telConfig.CirconusCheckID
		circonusCfg.CheckManager.Check.ForceMetricActivation = telConfig.CirconusCheckForceMetricActivation
		circonusCfg.CheckManager.Check.InstanceID = telConfig.CirconusCheckInstanceID
		circonusCfg.CheckManager.Check.SearchTag = telConfig.CirconusCheckSearchTag
		circonusCfg.CheckManager.Check.Tags = telConfig.CirconusCheckTags
		circonusCfg.CheckManager.Check.DisplayName = telConfig.CirconusCheckDisplayName
		circonusCfg.CheckManager.Broker.ID = telConfig.CirconusBrokerID
		circonusCfg.CheckManager.Broker.SelectTag = telConfig.CirconusBrokerSelectTag

		if circonusCfg.CheckManager.Check.DisplayName == "" {
			circonusCfg.CheckManager.Check.DisplayName = "Nomad Autoscaler"
		}
		if circonusCfg.CheckManager.API.TokenApp == "" {
			circonusCfg.CheckManager.API.TokenApp = "nomad-autoscaler"
		}
		if circonusCfg.CheckManager.Check.SearchTag == "" {
			circonusCfg.CheckManager.Check.SearchTag = "service:nomad-autoscaler"
		}

		sink, err := circonus.NewCirconusSink(circonusCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to setup Circonus sink: %v", err)
		}
		sink.Start()
		fanout = append(fanout, sink)
	}

	// Add the in-memory sink to the fanout.
	fanout = append(fanout, inm)

	// Initialize the global sink.
	_, err := metrics.NewGlobal(metricsConf, fanout)
	if err != nil {
		return nil, fmt.Errorf("failed to setup global sink: %v", err)
	}
	return inm, nil
}
