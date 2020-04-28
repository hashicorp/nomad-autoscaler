package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
)

func validateScalingPolicy(policy *api.ScalingPolicy) error {
	var result *multierror.Error

	if policy == nil {
		result = multierror.Append(result, fmt.Errorf("ScalingPolicy is nil"))
		return result
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

	// Validate Target
	if targetErr := validateTarget(policy.Target); targetErr != nil {
		result = multierror.Append(result, targetErr)
	}

	// Validate Policy
	if policyErr := validatePolicy(policy.Policy); policyErr != nil {
		result = multierror.Append(result, policyErr)
	}

	return result.ErrorOrNil()
}

func validateTarget(t map[string]string) error {
	const path = "ScalingPolicy.Target"

	var result *multierror.Error

	if t == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	// Validate required keys are defined.
	requiredKeys := []string{"Job", "Group"}
	for _, k := range requiredKeys {
		if v := t[k]; v == "" {
			result = multierror.Append(result, fmt.Errorf(`%s is missing key "%s"`, path, k))
		}
	}

	return result.ErrorOrNil()
}

func validatePolicy(p map[string]interface{}) error {
	const path = "ScalingPolicy.Policy"

	var result *multierror.Error

	if p == nil {
		return multierror.Append(result, fmt.Errorf("%s is nil", path))
	}

	// Validate Source.
	//   1. Source value must be a string if defined.
	source, ok := p[keySource]
	if ok {
		sourceString, ok := source.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s[%s] must be string, found %T", path, keySource, sourceString))
		}
	}

	// Validate Query.
	//   1. Query key must exist.
	//   2. Query must have string value.
	//   3. Query must not be empty.
	query, ok := p[keyQuery]
	if !ok {
		result = multierror.Append(result, fmt.Errorf(`%s is missing key "%s"`, path, keyQuery))
	} else {
		queryStr, ok := query.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s[%s] must be string, found %T", path, keyQuery, queryStr))
		} else {
			if queryStr == "" {
				result = multierror.Append(result, fmt.Errorf("%s[%s] can't be empty", path, keyQuery))
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
			result = multierror.Append(result, fmt.Errorf("%s[%s] must be string, found %T", path, keyEvaluationInterval, evalIntervalString))
		} else {
			if _, err := time.ParseDuration(evalIntervalString); err != nil {
				result = multierror.Append(result, fmt.Errorf("%s[%s] must have time.Duration format", keyEvaluationInterval, evalInterval))
			}
		}
	}

	// Validate Strategy.
	//   1. Strategy key must exist.
	//   2. Strategy must have []interface{} value.
	//        This is due the way HCL parses blocks, it creates a list to avoid
	//        overwriting blocks of the same type.
	//   3. Strategy must have just one element.
	//   4. The element in Strategy must be of type map[string]interface{}
	strategyInterface, ok := p[keyStrategy]
	if !ok {
		result = multierror.Append(result, fmt.Errorf(`%s missing key "%s"`, path, keyStrategy))
	} else {
		strategyList, ok := strategyInterface.([]interface{})
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s[%s] must be []interface{}, found %T", path, keyStrategy, strategyList))
		} else {
			if len(strategyList) != 1 {
				result = multierror.Append(result, fmt.Errorf("%s[%s] must have length 1, found %d", path, keyStrategy, len(strategyList)))
			} else {
				strategyMap, ok := strategyList[0].(map[string]interface{})
				if !ok {
					result = multierror.Append(result, fmt.Errorf("%s[%s][0] must be map[string]interface{}, found %T", path, keyStrategy, strategyMap))
				} else {
					if strategyErrs := validateStrategy(strategyMap); strategyErrs != nil {
						result = multierror.Append(result, strategyErrs)
					}
				}
			}
		}
	}

	return result.ErrorOrNil()
}

func validateStrategy(s map[string]interface{}) error {
	var path = fmt.Sprintf("ScalingPolicy.Policy[%s]", keyStrategy)

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
		result = multierror.Append(result, fmt.Errorf(`%s is missing key "%s"`, path, nameKey))
	} else {
		nameString, ok := nameInterface.(string)
		if !ok {
			result = multierror.Append(result, fmt.Errorf("%s[%s] must be string, found %T", path, nameKey, nameString))
		} else {
			if nameString == "" {
				result = multierror.Append(result, fmt.Errorf("%s[%s] can't be empty", path, nameKey))
			}
		}
	}

	return result.ErrorOrNil()
}
