package targetvalue

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/nomad-autoscaler/strategy"
)

type Strategy struct {
	config map[string]string
}

func (s *Strategy) SetConfig(config map[string]string) error {
	s.config = config
	return nil
}

func (s *Strategy) Run(req strategy.RunRequest) (strategy.RunResponse, error) {
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
		Count:  newCount,
		Reason: reason,
	}
	resp.Actions = append(resp.Actions, action)
	return resp, nil
}
