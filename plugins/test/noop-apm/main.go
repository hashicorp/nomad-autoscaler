package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	pluginName = "noop-apm"
)

var (
	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeAPM,
	}
)

var _ apm.APM = (*Noop)(nil)

type Noop struct {
	logger hclog.Logger
}

func (n *Noop) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	m, err := n.Query(q, r)
	if err != nil {
		return nil, err
	}
	return []sdk.TimestampedMetrics{m}, nil
}

func (n *Noop) Query(q string, r sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	n.logger.Debug("query request", "query", q, "range", r)

	var result sdk.TimestampedMetrics

	// Generate one value per second.
	repeat := int(r.To.Sub(r.From).Seconds())

	parts := strings.Split(q, ":")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid query %q", q)
	}

	switch parts[0] {
	case "fixed":
		if len(parts) != 2 {
			return nil, fmt.Errorf(`invalid fixed query %q, expected "fixed:<num>`, q)
		}

		num, err := strconv.ParseFloat(parts[1], 10)
		if err != nil {
			return nil, err
		}

		for i := 1; i <= repeat; i++ {
			ts := r.From.Add(time.Duration(i) * time.Second).UTC()
			result = append(result, sdk.TimestampedMetric{Value: num, Timestamp: ts})
		}
	case "random":
		if len(parts) != 3 {
			return nil, fmt.Errorf(`invalid random query %q, expected "random:<start>:<end>`, q)
		}

		start, err := strconv.ParseFloat(parts[1], 10)
		if err != nil {
			return nil, err
		}

		end, err := strconv.ParseFloat(parts[2], 10)
		if err != nil {
			return nil, err
		}

		rand.Seed(time.Now().UnixNano())
		for i := 1; i <= repeat; i++ {
			ts := r.From.Add(-time.Duration(i) * time.Second).UTC()
			value := start + rand.Float64()*(end-start)
			result = append(result, sdk.TimestampedMetric{Value: value, Timestamp: ts})
		}
	default:
		return nil, fmt.Errorf("invalid query type %q", parts[0])
	}

	n.logger.Trace("query result", "result", result)
	return result, nil
}

func (n *Noop) PluginInfo() (*base.PluginInfo, error) {
	n.logger.Debug("plugin info")
	return pluginInfo, nil
}

func (n *Noop) SetConfig(config map[string]string) error {
	n.logger.Debug("set config", "config", config)
	return nil
}

func main() {
	plugins.Serve(factory)
}

func factory(l hclog.Logger) interface{} {
	return &Noop{logger: l}
}
