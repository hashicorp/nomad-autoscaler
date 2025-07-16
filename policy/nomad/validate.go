// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad/api"
)

type validatorFunc func(in map[string]interface{}, path string) error
type validatorWithLabelFunc func(in map[string]interface{}, path string, label string) error

// nonMetricStrategies is a set of strategies that do not require metrics, so
// the `query` attribute is considered optional.
var nonMetricStrategies = map[string]bool{
	plugins.InternalStrategyFixedValue: true,
}

// validateScalingPolicy validates an api.ScalingPolicy object from the Nomad API
func validateScalingPolicy(policy *api.ScalingPolicy) error {
	var result *multierror.Error

	if policy == nil {
		return multierror.Append(result, errors.New("ScalingPolicy c"))
	}

	// Validate ID.
	if policy.ID == "" {
		result = multierror.Append(result, errors.New("ID is empty"))
	}

	// Validate Target.
	if policy.Target == nil {
		result = multierror.Append(result, errors.New("Target is nil")) //lint:ignore ST1005 Target is a field value
	}

	// Validate Policy.
	if policyErr := validatePolicy(policy.Policy); policyErr != nil {
		result = multierror.Append(result, policyErr)
	}

	// Run type-specific validations.
	result = multierror.Append(result, validateScalingPolicyByType(policy))

	return result.ErrorOrNil()
}

func validateScalingPolicyByType(policy *api.ScalingPolicy) error {
	switch policy.Type {
	case "horizontal", "":
		return validateHorizontalPolicy(policy)
	case "cluster":
		return validateClusterPolicy(policy)
	default:
		return additionalPolicyTypeValidation(policy)
	}
}

// validatePolicy validates the content of the policy block inside scaling.
//
//	scaling {
//	 +----------+
//	 | policy { |
//	 |   ...    |
//	 | }        |
//	 +----------+
//	}
func validatePolicy(p map[string]interface{}) error {
	const path = "scaling.policy"

	var result *multierror.Error

	if p == nil {
		return multierror.Append(result, fmt.Errorf("empty policy, this policy won't execute any verification or scaling and should have enabled set to false"))
	}

	// Validate EvaluationInterval, if present.
	//   1. EvaluationInterval should be a valid duration.
	if evalInterval, ok := p[keyEvaluationInterval]; ok {
		if err := validateDuration(evalInterval, path+"."+keyEvaluationInterval); err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Validate Cooldown, if present.
	//   1. Cooldown and cooldownOnScaleUp should be a valid duration.
	if cooldown, ok := p[keyCooldown]; ok {
		if err := validateDuration(cooldown, path+"."+keyCooldown); err != nil {
			result = multierror.Append(result, err)
		}
	}

	if cooldownOnScaleUp, ok := p[keyCooldownOnScaleUp]; ok {
		if err := validateDuration(cooldownOnScaleUp, path+"."+keyCooldownOnScaleUp); err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Validate Target, if present.
	if targetInterface, ok := p[keyTarget]; ok {
		err := validateBlocks(targetInterface, path+"."+keyTarget, validateTarget)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Validate Check blocks.
	err := validateBlocks(p[keyChecks], path+"."+keyChecks, validateChecks)
	if err != nil {
		result = multierror.Append(result, err)
	}

	return result.ErrorOrNil()
}

// validateTarget validates target blocks within policy.
//
//	scaling {
//	  policy {
//	  +-------------------+
//	  | target "target" { |
//	  |   key = "value"   |
//	  | }                 |
//	  +-------------------+
//	    }
//	  }
//	}
//
// Validation rules:
//  1. Only one target block at maxmimum.
//  2. Block must have a label.
//  3. Block structure should be valid.
func validateTarget(t map[string]interface{}, path string) error {
	return validateLabeledBlocks(t, path, nil, ptr.Of(1), nil)
}

// validateChecks validates the set of check blocks within policy.
//
//	scaling {
//	  policy {
//	  +-------------------+
//	  | check "check-1" { |
//	  |   ...             |
//	  | }                 |
//	  |                   |
//	  | check "check-2" { |
//	  |   ...             |
//	  | }                 |
//	  +-------------------+
//	    }
//	  }
//	}
//
// Validation rules:
//  1. At least one check block.
//  2. All check blocks should have labels.
//  3. All check blocks structure should be valid.
func validateChecks(in map[string]interface{}, path string) error {
	return validateLabeledBlocks(in, path, ptr.Of(1), nil, validateCheck)
}

// validateCheck validates the content of a check block.
//
//	scaling {
//	  policy {
//	    check "check" {
//	    +---------------+
//	    | key = "value" |
//	    +---------------+
//	    }
//	  }
//	}
func validateCheck(c map[string]interface{}, path string, label string) error {
	var result *multierror.Error

	if c == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	// Validate Source, if present.
	//   1. Source value must be a string if defined.
	source, sourceOk := c[keySource]
	if sourceOk {
		_, ok := source.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s.%s must be string, found %T", path, keySource, source))
		}
	}

	// Validate Query.
	//   1. Query must have string value.
	//   2. Query must not be empty.
	query, queryOk := c[keyQuery]
	if queryOk {
		queryStr, ok := query.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s.%s must be string, found %T", path, keyQuery, query))
		} else {
			if queryStr == "" {
				result = multierror.Append(result, fmt.Errorf("%s.%s can't be empty", path, keyQuery))
			}
		}
	}

	// Validate QueryWindow, if present.
	//   1. QueryWindow should be a valid time duration.
	queryWindow, ok := c[keyQueryWindow]
	if ok {
		if err := validateDuration(queryWindow, path+"."+keyQueryWindow); err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Some strategy plugins do not require an APM
	var strategyValidator validatorWithLabelFunc
	if !queryOk && !sourceOk {
		strategyValidator = validateStrategyWithoutMetric
	}

	// Validate Strategy.
	//   1. Strategy key must exist.
	//   2. Strategy must be a valid block.
	//   3. Only 1 Strategy allowed.
	//   4. Strategy block content must pass custom validator function.
	strategyValidatorWrapper := func(s map[string]interface{}, path string) error {
		return validateStrategy(s, path, strategyValidator)
	}
	strategyErrs := validateBlocks(c[keyStrategy], path+"."+keyStrategy, strategyValidatorWrapper)
	if strategyErrs != nil {
		result = multierror.Append(result, strategyErrs)
	}

	return result.ErrorOrNil()
}

// validateStrategy validates strategy blocks within a policy check.
//
//	scaling {
//	  policy {
//	    check "check" {
//	    +-----------------------+
//	    | strategy "strategy" { |
//	    |   key = "value"       |
//	    | }                     |
//	    +-----------------------+
//	    }
//	  }
//	}
//
// Validation rules:
//  1. Only one strategy block.
//  2. Block must have a label.
//  3. Block structure should be valid.
func validateStrategy(s map[string]interface{}, path string, validator validatorWithLabelFunc) error {
	return validateLabeledBlocks(s, path, ptr.Of(1), ptr.Of(1), validator)
}

// validateStrategyWithoutMetric validates strategy block contents for strategies
// that do not require an APM.
// It is called for checks that do not have `source` nor `query`.
//
//	scaling {
//	  policy {
//	    check "check" {
//	      strategy "strategy" {
//	      +---------------+
//	      | key = "value" |
//	      +---------------+
//	      }
//	    }
//	  }
//	}
//
// Validation rules:
//  1. Strategy does not require source
//  2. Strategy does not require query
func validateStrategyWithoutMetric(s map[string]interface{}, path string, label string) error {
	if _, ok := nonMetricStrategies[label]; ok {
		return nil
	}
	return fmt.Errorf("%s strategy requires a query", path)
}

// validateDuration validates if the input has a valid time.Duration format.
//
// Validation rules:
//  1. Input must be a string.
//  2. Input must parse to a time.Duration.
func validateDuration(d interface{}, path string) error {
	dStr, ok := d.(string)
	if !ok {
		return fmt.Errorf("%s must be string, found %T", path, d)
	}

	if _, err := time.ParseDuration(dStr); err != nil {
		return fmt.Errorf(`%s must have time.Duration format, found "%s"`, path, dStr)
	}

	return nil
}

// validateBlock validates the structure of a block parsed from HCL.
// The content of the block can be further validated by passing a `validator`
// function.
//
// Expected input format:
//
//	[]interface{} {
//	  map[string]interface{} {
//	    "key": interface{}
//	  }
//	}
func validateBlock(in interface{}, path string, validator validatorFunc) error {
	var result *multierror.Error

	runValidator := func(input map[string]interface{}) {
		if validator == nil {
			return
		}
		if err := validator(input, path); err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Return early if input already has the expected nested type.
	if inMap, ok := in.(map[string]interface{}); ok {
		runValidator(inMap)
		return result.ErrorOrNil()
	}

	list, ok := in.([]interface{})
	if !ok {
		return multierror.Append(result, fmt.Errorf("%s must be []interface{}, found %T", path, in))
	}

	if len(list) != 1 {
		return multierror.Append(result, fmt.Errorf("%s must have length 1, found %d", path, len(list)))
	}

	inMap, ok := list[0].(map[string]interface{})
	if !ok {
		return multierror.Append(result, fmt.Errorf("%s[0] must be map[string]interface{}, found %T", path, list[0]))
	}

	runValidator(inMap)

	return result.ErrorOrNil()
}

// validateBlocks validates the expected structure of a list of blocks of the
// same type.
// It flattens the list of blocks into a map and passes it to the validator
// function for further validation of each blocks' content.
//
// Expected input format:
//
//	[]interface{} {
//	  map[string]interface{} {
//	    "block-type": []interface{} {
//	      map[string]interface{} {
//	        "label-1": []interface{} {
//	          map[string]interface{} {
//	            "key": interface{}
//	          }
//	        }
//	        "label-2": []interface{} {
//	          map[string]interface{} {
//	            "key-1": interface{}
//	            "key-2": interface{}
//	          }
//	        }
//	      }
//	    }
//	  }
//	}
func validateBlocks(in interface{}, path string, validator validatorFunc) error {
	var result *multierror.Error

	if in == nil {
		return multierror.Append(result, fmt.Errorf("empty checks, this policy won't execute any verification or scaling and should have enabled set to false"))
	}

	inList, ok := in.([]interface{})
	if !ok {
		return multierror.Append(result, fmt.Errorf("%s must be []interface{}, found %T", path, in))
	}

	blocksMap := make(map[string]interface{})
	for i, blockInterface := range inList {
		blockMap, ok := blockInterface.(map[string]interface{})
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s[%d] must be map[string]interface{}, found: %T", path, i, blockInterface))
			continue
		}

		for k, v := range blockMap {
			blocksMap[k] = v
		}
	}

	if validator != nil {
		if err := validator(blocksMap, path); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

// validateLabeledBlocks validates a set of labeled blocks.
// The min and max number of expected blocks can be defined using the `min`
// `max` parameters.
// The content of each block is further validated by passing it to the
// `validator` function.
//
// Expected input format:
//
//	map[string]interface{} {
//	  "label-1": []interface{} {
//	    map[string]interface{} {
//	      "key": interface{}
//	    }
//	  }
//	  "label-2": []interface{} {
//	    map[string]interface{} {
//	      "key-1": interface{}
//	      "key-2": interface{}
//	    }
//	  }
//	}
func validateLabeledBlocks(b map[string]interface{}, path string, min, max *int, validator validatorWithLabelFunc) error {
	var result *multierror.Error

	if b == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	if len(b) == 0 {
		return nil
	}

	var expected *int
	if min != nil && len(b) < *min {
		expected = min
	} else if max != nil && len(b) > *max {
		expected = max
	}
	if expected != nil {
		return multierror.Append(result, fmt.Errorf("expected %d %s block, found %d", *expected, path, len(b)))
	}

	for name, block := range b {
		// Validate name.
		//   1. Name must not be empty.
		if name == "" {
			result = multierror.Append(result, fmt.Errorf("block %s must have a label", path))
		}

		// Validate block content.
		//   1. Content must be a block.
		validatorWrapper := func(in map[string]interface{}, path string) error {
			if validator == nil {
				return nil
			}
			return validator(in, path, name)
		}
		if err := validateBlock(block, fmt.Sprintf("%s[%s]", path, name), validatorWrapper); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
