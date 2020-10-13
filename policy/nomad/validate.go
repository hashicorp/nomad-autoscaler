package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/sdk/helper/ptr"
	"github.com/hashicorp/nomad/api"
)

type validatorFunc func(in map[string]interface{}, path string) error

// validateScalingPolicy validates an api.ScalingPolicy object from the Nomad API
func validateScalingPolicy(policy *api.ScalingPolicy) error {
	var result *multierror.Error

	if policy == nil {
		return multierror.Append(result, fmt.Errorf("ScalingPolicy is nil"))
	}

	// Validate ID.
	if policy.ID == "" {
		result = multierror.Append(result, fmt.Errorf("ID is empty"))
	}

	// Validate Min and Max values.
	//   1. Min must not be nil.
	//   2. Min must be positive.
	//   3. Max must be positive.
	//   4. Max must not be nil.
	//   5. Min must be smaller than Max.
	if policy.Min == nil {
		result = multierror.Append(result, fmt.Errorf("scaling.min is missing"))
	} else if policy.Max == nil {
		result = multierror.Append(result, fmt.Errorf("scaling.max is missing"))
	} else {
		min := *policy.Min
		if min < 0 {
			result = multierror.Append(result, fmt.Errorf("scaling.min can't be negative"))
		}

		if min > *policy.Max {
			result = multierror.Append(result, fmt.Errorf("scaling.min must be smaller than scaling.max"))
		}

		if *policy.Max < 0 {
			result = multierror.Append(result, fmt.Errorf("scaling.max can't be negative"))
		}
	}

	// Validate Target.
	if policy.Target == nil {
		result = multierror.Append(result, fmt.Errorf("Target is nil"))
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
	const path = "scaling.policy"

	var result *multierror.Error

	if p == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	// Validate EvaluationInterval, if present.
	//   1. EvaluationInterval should be a valid duration.
	if evalInterval, ok := p[keyEvaluationInterval]; ok {
		if err := validateDuration(evalInterval, path+"."+keyEvaluationInterval); err != nil {
			result = multierror.Append(result, err)
		}
	}

	// Validate Cooldown, if present.
	//   1. Cooldown should be a valid duration.
	if cooldown, ok := p[keyCooldown]; ok {
		if err := validateDuration(cooldown, path+"."+keyCooldown); err != nil {
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
//  scaling {
//    policy {
//    +-------------------+
//    | target "target" { |
//    |   key = "value"   |
//    | }                 |
//    +-------------------+
//      }
//    }
//  }
//
// Validation rules:
//   1. Only one target block at maxmimum.
//   2. Block must have a label.
//   3. Block structure should be valid.
func validateTarget(t map[string]interface{}, path string) error {
	return validateLabeledBlocks(t, path, nil, ptr.IntToPtr(1), nil)
}

// validateChecks validates the set of check blocks within policy.
//
//  scaling {
//    policy {
//    +-------------------+
//    | check "check-1" { |
//    |   ...             |
//    | }                 |
//    |                   |
//    | check "check-2" { |
//    |   ...             |
//    | }                 |
//    +-------------------+
//      }
//    }
//  }
//
// Validation rules:
//   1. At least one check block.
//   2. All check blocks should have labels.
//   3. All check blocks structure should be valid.
func validateChecks(in map[string]interface{}, path string) error {
	return validateLabeledBlocks(in, path, ptr.IntToPtr(1), nil, validateCheck)
}

// validateCheck validates the content of a check block.
//
//  scaling {
//    policy {
//      check "check" {
//      +---------------+
//      | key = "value" |
//      +---------------+
//      }
//    }
//  }
func validateCheck(c map[string]interface{}, path string) error {
	var result *multierror.Error

	if c == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	// Validate Source, if present.
	//   1. Source value must be a string if defined.
	source, ok := c[keySource]
	if ok {
		_, ok := source.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s.%s must be string, found %T", path, keySource, source))
		}
	}

	// Validate Query.
	//   1. Query key must exist.
	//   2. Query must have string value.
	//   3. Query must not be empty.
	query, ok := c[keyQuery]
	if !ok {
		result = multierror.Append(result, fmt.Errorf("%s.%s is missing", path, keyQuery))
	} else {
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

	// Validate Strategy.
	//   1. Strategy key must exist.
	//   2. Strategy must be a valid block.
	//   3. Only 1 Strategy allowed.
	strategyErrs := validateBlocks(c[keyStrategy], path+"."+keyStrategy, validateStrategy)
	if strategyErrs != nil {
		result = multierror.Append(result, strategyErrs)
	}

	return result.ErrorOrNil()
}

// validateStrategy validates strategy blocks within a policy check.
//
//  scaling {
//    policy {
//      check "check" {
//      +-----------------------+
//      | strategy "strategy" { |
//      |   key = "value"       |
//      | }                     |
//      +-----------------------+
//      }
//    }
//  }
//
// Validation rules:
//   1. Only one strategy block.
//   2. Block must have a label.
//   3. Block structure should be valid.
func validateStrategy(s map[string]interface{}, path string) error {
	return validateLabeledBlocks(s, path, ptr.IntToPtr(1), ptr.IntToPtr(1), nil)
}

// validateDuration validates if the input has a valid time.Duration format.
//
// Validation rules:
//   1. Input must be a string.
//   2. Input must parse to a time.Duration.
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
//   []interface{} {
//     map[string]interface{} {
//       "key": interface{}
//     }
//   }
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
//   []interface{} {
//     map[string]interface{} {
//       "block-type": []interface{} {
//         map[string]interface{} {
//           "label-1": []interface{} {
//             map[string]interface{} {
//               "key": interface{}
//             }
//           }
//           "label-2": []interface{} {
//             map[string]interface{} {
//               "key-1": interface{}
//               "key-2": interface{}
//             }
//           }
//         }
//       }
//     }
//   }
func validateBlocks(in interface{}, path string, validator validatorFunc) error {
	var result *multierror.Error

	if in == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
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
//   map[string]interface{} {
//     "label-1": []interface{} {
//       map[string]interface{} {
//         "key": interface{}
//       }
//     }
//     "label-2": []interface{} {
//       map[string]interface{} {
//         "key-1": interface{}
//         "key-2": interface{}
//       }
//     }
//   }
func validateLabeledBlocks(b map[string]interface{}, path string, min, max *int, validator validatorFunc) error {
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
		if err := validateBlock(block, fmt.Sprintf("%s[%s]", path, name), validator); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
