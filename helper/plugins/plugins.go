package plugins

import (
	"os"
	"path"

	"github.com/hashicorp/nomad-autoscaler/apm"
	nomadapm "github.com/hashicorp/nomad-autoscaler/plugins/nomad/apm"
	nomadtarget "github.com/hashicorp/nomad-autoscaler/plugins/nomad/target"
	prometheus "github.com/hashicorp/nomad-autoscaler/plugins/prometheus/apm"
	targetvalue "github.com/hashicorp/nomad-autoscaler/plugins/target-value/strategy"
	"github.com/hashicorp/nomad-autoscaler/strategy"
	"github.com/hashicorp/nomad-autoscaler/target"
)

const (
	nomadAPM            = "nomad-apm"
	nomadTarget         = "nomad"
	prometheusAPM       = "prometheus"
	targetValueStrategy = "target-value"
)

func IsInternal(driver, pluginDir string) bool {
	// Use a plugin binary if one is available
	if _, err := os.Stat(path.Join(pluginDir, driver)); err == nil {
		return false
	}

	switch driver {
	case
		nomadAPM,
		nomadTarget,
		prometheusAPM,
		targetValueStrategy:
		return true
	}
	return false
}

func NewInternalAPM(driver string) apm.APM {
	switch driver {
	case nomadAPM:
		return &nomadapm.MetricsAPM{}
	case prometheusAPM:
		return &prometheus.APM{}
	}
	return nil
}

func NewInternalStrategy(driver string) strategy.Strategy {
	switch driver {
	case targetValueStrategy:
		return &targetvalue.Strategy{}
	}
	return nil
}

func NewInternalTarget(driver string) target.Target {
	switch driver {
	case nomadTarget:
		return &nomadtarget.NomadGroupCount{}
	}
	return nil
}
