package plugin

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
)

const (
	// pluginName is the name of the plugin
	pluginName = "target-value"
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeStrategy,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewTargetValuePlugin(l) },
	}

	pluginInfo = &plugins.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeStrategy,
	}
)

type StrategyPlugin struct {
	config map[string]string
	logger hclog.Logger
}

func NewTargetValuePlugin(log hclog.Logger) strategy.Strategy {
	return &StrategyPlugin{
		logger: log,
	}
}

func (s *StrategyPlugin) SetConfig(config map[string]string) error {
	s.config = config
	return nil
}

func (s *StrategyPlugin) PluginInfo() (*plugins.PluginInfo, error) {
	return pluginInfo, nil
}

func (s *StrategyPlugin) Run(req strategy.RunRequest) (strategy.RunResponse, error) {
	resp := strategy.RunResponse{Actions: []strategy.Action{}}

	t := req.Config["target"]
	if t == "" {
		return resp, fmt.Errorf("missing required field `target`")
	}

	target, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return resp, fmt.Errorf("invalid value for `target`: %v (%T)", t, t)
	}

	var reason, direction string
	factor := req.Metric / target

	if factor < 1 {
		direction = "down"
	} else if factor > 1 {
		direction = "up"
	} else {
		// factor is 1, no need to scale
		return resp, nil
	}

	reason = fmt.Sprintf("scaling %s because factor is %f", direction, factor)
	newCount := int64(math.Ceil(float64(req.Count) * factor))

	if newCount == req.Count {
		// count didn't change, no need to scale
		return resp, nil
	}

	action := strategy.Action{
		Count:  &newCount,
		Reason: reason,
	}
	resp.Actions = append(resp.Actions, action)
	return resp, nil
}
