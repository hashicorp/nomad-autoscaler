package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad/api"
)

// parsePolicy parses the values on an api.ScalingPolicy into a policy.Policy.
//
// It provides best-effort parsing, with any invalid values being skipped from
// the end result. To avoid missing values use validateScalingPolicy() to
// detect errors first.
func parsePolicy(p *api.ScalingPolicy) policy.Policy {
	to := policy.Policy{
		ID:       p.ID,
		Max:      p.Max,
		Enabled:  true,
		Target:   parseTarget(p.Policy[keyTarget], p.Target),
		Strategy: parseStrategy(p.Policy[keyStrategy]),
	}

	// Add non-typed values.
	if p.Min != nil {
		to.Min = *p.Min
	}

	if query, ok := p.Policy[keyQuery].(string); ok {
		to.Query = query
	}

	if source, ok := p.Policy[keySource].(string); ok {
		to.Source = source
	}

	if p.Enabled != nil {
		to.Enabled = *p.Enabled
	}

	// Parse evaluation_interval as time.Duration.
	// Ignore error since we assume policy has been validated.
	if eval, ok := p.Policy[keyEvaluationInterval].(string); ok {
		to.EvaluationInterval, _ = time.ParseDuration(eval)
	}

	return to
}

// parseStrategy parses the content of the strategy block from a policy.
//
// It provides best-effort parsing and will return `nil` in case of errors.
//
//  scaling {
//    policy {
//      strategy = {
//      +-------------------+
//      | name = "strategy" |
//      | config = {        |
//      |   key = "value"   |
//      | }                 |
//      +-------------------+
//      }
//    }
//  }
func parseStrategy(s interface{}) *policy.Strategy {
	if s == nil {
		return nil
	}

	strategyMap := parseBlock(s)
	if strategyMap == nil {
		return nil
	}

	var configMapString map[string]string
	configMap := parseBlock(strategyMap["config"])

	if configMap != nil {
		configMapString = make(map[string]string)
		for k, v := range configMap {
			configMapString[k] = fmt.Sprintf("%v", v)
		}
	}

	// Ignore ok, but we need _ to avoid panics.
	name, _ := strategyMap["name"].(string)

	return &policy.Strategy{
		Name:   name,
		Config: configMapString,
	}
}

// parseTarget parses the content of the target block from a policy and
// enhances it with values defined in Target as well. Values in target.config
// takes precedence over values in Target.
//
// It provides best-effort parsing and will return `nil` in case of errors.
//
//  scaling {
//    policy {
//      target = {
//      +-----------------+
//      | name = "target" |
//      | config = {      |
//      |   key = "value" |
//      | }               |
//      +-----------------+
//      }
//    }
//  }
func parseTarget(targetBlock interface{}, targetAttr map[string]string) *policy.Target {

	targetMap := parseBlock(targetBlock)
	if targetMap == nil && targetAttr == nil {
		return nil
	}

	configMapString := make(map[string]string)
	for k, v := range targetAttr {
		configMapString[k] = v
	}
	if targetMap != nil {
		configMap := parseBlock(targetMap["config"])

		if configMap != nil {
			for k, v := range configMap {
				configMapString[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	// Ignore ok, but we need _ to avoid panics.
	name, _ := targetMap["name"].(string)

	return &policy.Target{
		Name:   name,
		Config: configMapString,
	}
}

// parseBlock parses the specific structure of a block into a more usable
// value of map[string]interface{}.
func parseBlock(block interface{}) map[string]interface{} {
	blockInterfaceList, ok := block.([]interface{})
	if !ok || len(blockInterfaceList) != 1 {
		return nil
	}

	blockMap, ok := blockInterfaceList[0].(map[string]interface{})
	if !ok {
		return nil
	}

	return blockMap
}
