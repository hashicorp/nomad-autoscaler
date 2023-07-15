// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package target

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// Target is the interface that all Target plugins are required to implement.
// The plugins are responsible for providing status details of the remote
// target, as well as carrying out scaling actions as decided by the Strategy
// plugin and internal autoscaler controls.
type Target interface {

	// Embed base.Base ensuring that strategy plugins implement this interface.
	base.Base

	// Scale triggers a scaling action against the remote target as specified
	// by the config func argument.
	Scale(action sdk.ScalingAction, config map[string]string) error

	// Status collects and returns critical information of the status of the
	// remote target. The information is used to understand whether the target
	// is in a position to be scaled as well as the current running count which
	// will be used when performing the strategy calculation.
	Status(config map[string]string) (*sdk.TargetStatus, error)
}

type TelemetryTarget struct {
	name   string
	plugin Target
	labels []metrics.Label
}

func NewTelemetryTarget(name string, plugin Target) Target {
	return &TelemetryTarget{
		name:   name,
		plugin: plugin,
		labels: []metrics.Label{{
			Name: "plugin_name", Value: name,
		}},
	}
}

func (t *TelemetryTarget) PluginInfo() (*base.PluginInfo, error) {
	return t.plugin.PluginInfo()
}

func (t *TelemetryTarget) SetConfig(config map[string]string) error {
	return t.plugin.SetConfig(config)
}

func (t *TelemetryTarget) Scale(action sdk.ScalingAction, config map[string]string) error {
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "scale", "invoke_ms"}, time.Now(), t.labels)

	err := t.plugin.Scale(action, config)
	if err != nil {
		metrics.IncrCounterWithLabels([]string{"plugin", "target", "scale", "invoke", "error_count"}, 1, t.labels)
	} else {
		metrics.IncrCounterWithLabels([]string{"plugin", "target", "scale", "invoke", "success_count"}, 1, t.labels)
	}

	return err
}

func (t *TelemetryTarget) Status(config map[string]string) (*sdk.TargetStatus, error) {
	defer metrics.MeasureSinceWithLabels([]string{"plugin", "target", "status", "invoke_ms"}, time.Now(), t.labels)

	status, err := t.plugin.Status(config)
	if err != nil {
		metrics.IncrCounterWithLabels([]string{"plugin", "target", "status", "invoke", "error_count"}, 1, t.labels)
	} else {
		metrics.IncrCounterWithLabels([]string{"plugin", "target", "status", "invoke", "success_count"}, 1, t.labels)
	}

	return status, err
}
