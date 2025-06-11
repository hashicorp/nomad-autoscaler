package main

import (
	"strconv"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/pkg/errors"
)

const (
	pluginName = "static-count-apm"
)

var (
	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}
)

var _ apm.APM = (*StaticCount)(nil)

type StaticCount struct {
	logger hclog.Logger
}

func (n *StaticCount) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	m, err := n.Query(q, r)
	if err != nil {
		return nil, err
	}

	return []sdk.TimestampedMetrics{m}, nil
}

func (n *StaticCount) Query(q string, r sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	n.logger.Debug("query request", "query", q, "range", r)

	val, err := strconv.Atoi(q)
	if err != nil {
		return nil, errors.Wrapf(err, "query must be an integer but %s was provided", q)
	}

	n.logger.Debug("query response", "query", q, "total", val)

	return sdk.TimestampedMetrics{
		{
			Timestamp: time.Now(),
			Value:     float64(val),
		},
	}, nil

}

func (n *StaticCount) PluginInfo() (*base.PluginInfo, error) {
	n.logger.Debug("plugin info")
	return pluginInfo, nil
}

func (n *StaticCount) SetConfig(config map[string]string) error {
	return nil
}

func main() {
	plugins.Serve(factory)
}

func factory(l hclog.Logger) interface{} {
	return &StaticCount{logger: l}
}
