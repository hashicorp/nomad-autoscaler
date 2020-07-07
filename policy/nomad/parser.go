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
	if p == nil {
		return policy.Policy{}
	}

	to := policy.Policy{
		ID:      p.ID,
		Max:     *p.Max, // Nomad always ensures Max is populated.
		Enabled: true,
		Checks:  parseChecks(p.Policy[keyChecks]),
	}

	// Add non-typed values.
	if p.Min != nil {
		to.Min = *p.Min
	}

	if p.Enabled != nil {
		to.Enabled = *p.Enabled
	}

	// Parse evaluation_interval as time.Duration.
	// Ignore error since we assume policy has been validated.
	if eval, ok := p.Policy[keyEvaluationInterval].(string); ok {
		to.EvaluationInterval, _ = time.ParseDuration(eval)
	}

	// Parse cooldown as time.Duraction
	// Ignore error since we assume policy has been validated.
	if cooldown, ok := p.Policy[keyCooldown].(string); ok {
		to.Cooldown, _ = time.ParseDuration(cooldown)
	}

	// Parse target block.
	var target *policy.Target

	if p.Policy[keyTarget] == nil {
		// Target was not specified in the policy block, but parse values from
		// the Target field.
		target = parseTarget(nil, p.Target)
	} else {
		// There shouldn't be more than one, but iterate just in case.
		for k, v := range parseBlocks(p.Policy[keyTarget]) {
			target = parseTarget(v, p.Target)
			if target != nil {
				target.Name = k
				break
			}
		}
	}
	to.Target = target

	return to
}

// parseChecks parses the list of checks in a scaling policy.
//
// It provides best-effort parsing and will return `nil` in case of errors.
func parseChecks(cs interface{}) []*policy.Check {
	if cs == nil {
		return nil
	}

	checksInterfaceList, ok := cs.([]interface{})
	if !ok {
		return nil
	}

	var checks []*policy.Check
	checksBlocks := parseBlocks(checksInterfaceList)

	for k, v := range checksBlocks {
		check := parseCheck(v)
		if check != nil {
			check.Name = k
			checks = append(checks, check)
		}
	}

	return checks
}

// parseCheck parses the content of a check block from a policy.
//
// It provides best-effort parsing and will return `nil` in case of errors.
//
//  scaling {
//    policy {
//    +--------------------------------+
//    | check "name" {                 |
//    |   source = "source"            |
//    |   query = "query"              |
//    |   strategy "strategy" { ... }  |
//    | }                              |
//    +--------------------------------+
//    }
//  }
func parseCheck(c interface{}) *policy.Check {
	if c == nil {
		return nil
	}

	checkMap := parseBlock(c)
	if checkMap == nil {
		return nil
	}

	// Parse a single strategy block.
	// There shouldn't be more than one, but iterate just in case.
	var strategy *policy.Strategy
	for k, v := range parseBlocks(checkMap[keyStrategy]) {
		strategy = parseStrategy(v)
		if strategy != nil {
			strategy.Name = k
			break
		}
	}

	// Parse query and source with _ to avoid panics.
	query, _ := checkMap[keyQuery].(string)
	source, _ := checkMap[keySource].(string)

	return &policy.Check{
		Query:    query,
		Source:   source,
		Strategy: strategy,
	}
}

// parseStrategy parses the content of the strategy block from a policy.
//
// It provides best-effort parsing and will return `nil` in case of errors.
//
//  scaling {
//    policy {
//      strategy "strategy" {
//      +---------------+
//      | key = "value" |
//      +---------------+
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

	configMapString := make(map[string]string)
	for k, v := range strategyMap {
		configMapString[k] = fmt.Sprintf("%v", v)
	}

	return &policy.Strategy{
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
//      target "target"  {
//      +---------------+
//      | key = "value" |
//      +---------------+
//      }
//    }
//  }
func parseTarget(targetBlock interface{}, targetAttr map[string]string) *policy.Target {
	targetMap := parseBlock(targetBlock)
	if targetMap == nil && targetAttr == nil {
		return nil
	}

	// Copy values from api.ScalingPolicy.Target.
	configMapString := make(map[string]string)
	for k, v := range targetAttr {
		configMapString[k] = v
	}

	if targetMap != nil {
		for k, v := range targetMap {
			configMapString[k] = fmt.Sprintf("%v", v)
		}
	}

	return &policy.Target{
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

// parseBlocks flattens a list of block into a map, with the labels as keys.
func parseBlocks(blocks interface{}) map[string]interface{} {
	blocksInterfaceList, ok := blocks.([]interface{})
	if !ok {
		return nil
	}

	blocksMap := make(map[string]interface{})

	for _, blockInterface := range blocksInterfaceList {
		blockMap, ok := blockInterface.(map[string]interface{})
		if !ok {
			continue
		}

		for blockName, blockContent := range blockMap {
			blocksMap[blockName] = blockContent
		}
	}

	return blocksMap
}
