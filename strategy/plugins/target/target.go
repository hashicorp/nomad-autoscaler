package targetstrategy

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/strategy"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "magic",
	MagicCookieValue: "magic",
}

type TargetStrategy struct {
	Config map[string]string
}

func (s *TargetStrategy) Run(req *strategy.RunRequest) ([]strategy.Action, error) {
	target := req.Config["target"]
	if target == "" {
		return nil, fmt.Errorf("missing required field `target`")
	}

	c, err := strconv.ParseFloat(target, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid value for `target`: %v (%T)", target, target)
	}

	var reason, direction string
	factor := req.CurrentValue / c

	if factor < 0 {
		direction = "down"
	} else if factor > 1 {
		direction = "up"
	}
	if direction != "" {
		reason = fmt.Sprintf("scaling %s because factor is %f", direction, factor)
	} else {
		// factor is 1, no need to scale
		return []strategy.Action{}, nil
	}

	newCount := int(math.Ceil(float64(req.CurrentCount) * factor))
	if newCount < req.MinCount {
		newCount = req.MinCount
	} else if newCount > req.MaxCount {
		newCount = req.MaxCount
	}

	if newCount == req.CurrentCount {
		// count didn't change, no need to scale
		return []strategy.Action{}, nil
	}

	action := strategy.Action{
		TargetID: req.TargetID,
		Count:    newCount,
		Reason:   reason,
	}
	return []strategy.Action{action}, nil
}

func main() {
	s := &TargetStrategy{}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"strategy": &strategy.Plugin{Impl: s},
		},
	})

}
