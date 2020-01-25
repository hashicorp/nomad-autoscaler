package target

import "github.com/hashicorp/nomad-autoscaler/strategy"

type Target interface {
	Count(config map[string]string) (int, error)
	Scale(actions []strategy.Action, config map[string]string) error
}
