package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
)

// validateScalingPolicy validates an api.ScalingPolicy object from the Nomad API
func validateScalingPolicy(policy *api.ScalingPolicy) error {
	var result *multierror.Error

	if policy == nil {
		result = multierror.Append(result, fmt.Errorf("ScalingPolicy is nil"))
		return result
	}

	// Validate ID.
	if policy.ID == "" {
		result = multierror.Append(result, fmt.Errorf("ScalingPolicy.ID is empty"))
	}

	// Validate Min and Max values.
	//   1. Min must not be nil.
	//   2. Min must be positive.
	//   3. Max must be positive.
	//   4. Min must be smaller than Max.
	if policy.Min == nil {
		result = multierror.Append(result, fmt.Errorf("ScalingPolicy.Min is nil"))
	} else {
		min := *policy.Min
		if min < 0 {
			result = multierror.Append(result, fmt.Errorf("ScalingPolicy.Min can't be negative"))
		}

		if min > policy.Max {
			result = multierror.Append(result, fmt.Errorf("ScalingPolicy.Min must be smaller than ScalingPolicy.Max"))
		}
	}

	if policy.Max < 0 {
		result = multierror.Append(result, fmt.Errorf("ScalingPolicy.Max can't be negative"))
	}

	// Validate Target.
	if policy.Target == nil {
		result = multierror.Append(result, fmt.Errorf("ScalingPolicy.Target is nil"))
	}

	// Validate Policy.
	if policyErr := validatePolicy(policy.Policy); policyErr != nil {
		result = multierror.Append(result, policyErr)
	}

	return result.ErrorOrNil()
}

// validatePolicy validates the content of the policy block inside scaling.
//
//  scaling {
//   +----------+
//   | policy { |
//   |   ...    |
//   | }        |
//   +----------+
//  }
func validatePolicy(p map[string]interface{}) error {
	const path = "scaling->policy"

	var result *multierror.Error

	if p == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	// Validate Source (optional).
	//   1. Source value must be a string if defined.
	source, ok := p[keySource]
	if ok {
		_, ok := source.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s->%s must be string, found %T", path, keySource, source))
		}
	}

	// Validate Query.
	//   1. Query key must exist.
	//   2. Query must have string value.
	//   3. Query must not be empty.
	query, ok := p[keyQuery]
	if !ok {
		result = multierror.Append(result, fmt.Errorf("%s->%s is missing", path, keyQuery))
	} else {
		queryStr, ok := query.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s->%s must be string, found %T", path, keyQuery, query))
		} else {
			if queryStr == "" {
				result = multierror.Append(result, fmt.Errorf("%s->%s can't be empty", path, keyQuery))
			}
		}
	}

	// Validate EvaluationInterval.
	//   1. EvaluationInterval must have string value if defined.
	//   2. EvaluationInterval must have time.Duration format if defined.
	evalInterval, ok := p[keyEvaluationInterval]
	if ok {
		evalIntervalString, ok := evalInterval.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s->%s must be string, found %T", path, keyEvaluationInterval, evalInterval))
		} else {
			if _, err := time.ParseDuration(evalIntervalString); err != nil {
				result = multierror.Append(result, fmt.Errorf("%s->%s must have time.Duration format", path, keyEvaluationInterval))
			}
		}
	}

	// Validate Strategy.
	//   1. Strategy key must exist.
	//   2. Strategy must be a valid HCL block.
	strategyErrs := validateHCLBlock(p[keyStrategy], path, keyStrategy, validateStrategy)
	if strategyErrs != nil {
		result = multierror.Append(result, strategyErrs)
	}

	// Validate Target (optional).
	//   1. Target must be a valid HCL block if present.
	targetInterface, ok := p[keyTarget]
	if ok {
		targetErr := validateHCLBlock(targetInterface, path, keyTarget, validateTarget)
		if targetErr != nil {
			result = multierror.Append(result, targetErr)
		}
	}

	return result.ErrorOrNil()
}

// validateStrategy validates the content of the strategy block inside policy.
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
func validateStrategy(s map[string]interface{}) error {
	var path = fmt.Sprintf("scaling->policy->%s", keyStrategy)

	var result *multierror.Error

	if s == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	// Validate name.
	//   1. Name key must exist.
	//   2. Name must have string value.
	//   3. Name must not be empty.
	nameKey := "name"
	nameInterface, ok := s[nameKey]
	if !ok {
		result = multierror.Append(result, fmt.Errorf("%s->%s is missing", path, nameKey))
	} else {
		nameString, ok := nameInterface.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s->%s must be string, found %T", path, nameKey, nameInterface))
		} else {
			if nameString == "" {
				result = multierror.Append(result, fmt.Errorf("%s->%s can't be empty", path, nameKey))
			}
		}
	}

	// Validate config (optional).
	//   1. Config must be an HCL block if present.
	configKey := "config"
	if config, ok := s[configKey]; ok {
		err := validateHCLBlock(config, path, configKey, nil)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

// validateTarget validates the content of the target block inside policy.
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
func validateTarget(t map[string]interface{}) error {
	var path = fmt.Sprintf("scaling->policy->%s", keyTarget)

	var result *multierror.Error

	// Validate name (optional).
	//   1. Name must have string value if present.
	//   2. Name must not be empty if present.
	nameKey := "name"
	nameInterface, ok := t[nameKey]
	if ok {
		nameString, ok := nameInterface.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s->%s must be string, found %T", path, nameKey, nameInterface))
		} else {
			if nameString == "" {
				result = multierror.Append(result, fmt.Errorf("%s->%s can't be empty", path, nameKey))
			}
		}
	}

	// Validate config (optional).
	//   1. Config must be an HCL block if present.
	configKey := "config"
	if config, ok := t[configKey]; ok {
		err := validateHCLBlock(config, path, configKey, nil)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

// validateHCLBlock validates the kind of unusual structure we receive when the policy HCL block is parsed.
func validateHCLBlock(in interface{}, path, key string, validator func(in map[string]interface{}) error) error {
	var result *multierror.Error

	list, ok := in.([]interface{})
	if !ok {
		return multierror.Append(result, fmt.Errorf("%s->%s must be []interface{}, found %T", path, key, in))
	}

	if len(list) != 1 {
		return multierror.Append(result, fmt.Errorf("%s->%s must have length 1, found %d", path, key, len(list)))
	}

	inMap, ok := list[0].(map[string]interface{})
	if !ok {
		return multierror.Append(result, fmt.Errorf("%s->%s[0] must be map[string]interface{}, found %T", path, key, list[0]))
	}

	if validator != nil {
		if err := validator(inMap); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
