package targetvalue

import (
	"fmt"
	"math"
	"strconv"

	"github.com/hashicorp/nomad-autoscaler/strategy"
)

type TargetValue struct {
	config map[string]string
}

func (s *TargetValue) SetConfig(config map[string]string) error {
	return nil
}

func (s *TargetValue) Run(req strategy.RunRequest) (strategy.RunResponse, error) {
	resp := strategy.RunResponse{Actions: []strategy.Action{}}

	target := req.Config["target"]
	if target == "" {
		return resp, fmt.Errorf("missing required field `target`")
	}

	c, err := strconv.ParseFloat(target, 64)
	if err != nil {
		return resp, fmt.Errorf("invalid value for `target`: %v (%T)", target, target)
	}

	var reason, direction string
	factor := req.CurrentValue / c

	if factor < 1 {
		direction = "down"
	} else if factor > 1 {
		direction = "up"
	} else {
		// factor is 1, no need to scale
		return resp, nil
	}

	reason = fmt.Sprintf("scaling %s because factor is %f", direction, factor)
	newCount := int64(math.Ceil(float64(req.CurrentCount) * factor))
	if newCount < req.MinCount {
		newCount = req.MinCount
	} else if newCount > req.MaxCount {
		newCount = req.MaxCount
	}

	if newCount == req.CurrentCount {
		// count didn't change, no need to scale
		return resp, nil
	}

	action := strategy.Action{
		Count:  int(newCount),
		Reason: reason,
	}
	resp.Actions = append(resp.Actions, action)
	return resp, nil
}
