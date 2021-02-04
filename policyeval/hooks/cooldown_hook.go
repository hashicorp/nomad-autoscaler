package hooks

import (
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

type CooldownHook struct {
	logger    hclog.Logger
	cooldowns map[string]time.Time
	lock      sync.RWMutex
}

func NewCooldownHook() *CooldownHook {
	return &CooldownHook{
		logger:    hclog.Default().Named("hooks").With("hook", "cooldown"),
		cooldowns: make(map[string]time.Time),
	}
}

func (c *CooldownHook) PreStart(eval *sdk.ScalingEvaluation) error {
	c.lock.RLock()
	defer c.lock.RUnlock()

	fmt.Print("cooldown PreStart\n")

	if deadline, ok := c.cooldowns[eval.Policy.ID]; ok {
		if deadline.After(time.Now()) {
			c.logger.Debug("policy is in cooldown", "policy_id", eval.Policy.ID)
			return fmt.Errorf("policy in cooldown until %v", deadline)
		}
	}

	return nil
}

func (c *CooldownHook) PostStatus(eval *sdk.ScalingEvaluation) error {
	return nil
}

func (c *CooldownHook) PostQuery(eval *sdk.ScalingEvaluation) error {
	return nil
}

func (c *CooldownHook) PostStrategy(eval *sdk.ScalingEvaluation) error {
	return nil
}

func (c *CooldownHook) PreScale(eval *sdk.ScalingEvaluation) error {
	return nil
}

func (c *CooldownHook) PostScale(eval *sdk.ScalingEvaluation) error {
	fmt.Print("cooldown PostScale\n")

	if eval.Policy.Cooldown == 0 {
		return nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	deadline := time.Now().Add(eval.Policy.Cooldown)
	c.cooldowns[eval.Policy.ID] = deadline

	c.logger.Debug("policy placed in coold", "policy_id", eval.Policy.ID, "deadline", deadline)
	return nil
}
