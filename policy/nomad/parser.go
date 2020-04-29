package nomad

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad/api"
)

func parsePolicy(p *api.ScalingPolicy) (policy.Policy, error) {
	var to policy.Policy

	if err := validateScalingPolicy(p); err != nil {
		return to, err
	}

	source := p.Policy[keySource]
	if source == nil {
		source = ""
	}

	to = policy.Policy{
		ID:                 p.ID,
		Min:                *p.Min,
		Max:                p.Max,
		Enabled:            *p.Enabled,
		Source:             source.(string),
		Query:              p.Policy[keyQuery].(string),
		EvaluationInterval: defaultEvaluationInterval, //TODO(luiz): use agent scan interval as default
		Target:             parseTarget(p.Policy[keyTarget]),
		Strategy:           parseStrategy(p.Policy[keyStrategy]),
	}

	canonicalizePolicy(p, &to)

	return to, nil
}

func parseStrategy(s interface{}) *policy.Strategy {
	strategyMap := s.([]interface{})[0].(map[string]interface{})
	configMap := strategyMap["config"].([]interface{})[0].(map[string]interface{})
	configMapString := make(map[string]string)
	for k, v := range configMap {
		configMapString[k] = fmt.Sprintf("%v", v)
	}

	return &policy.Strategy{
		Name:   strategyMap["name"].(string),
		Config: configMapString,
	}
}

func parseTarget(t interface{}) *policy.Target {
	if t == nil {
		return &policy.Target{}
	}

	targetMap := t.([]interface{})[0].(map[string]interface{})
	if targetMap == nil {
		return &policy.Target{}
	}

	var configMapString map[string]string
	if targetMap["config"] != nil {
		configMap := targetMap["config"].([]interface{})[0].(map[string]interface{})
		configMapString = make(map[string]string)
		for k, v := range configMap {
			configMapString[k] = fmt.Sprintf("%v", v)
		}
	}
	return &policy.Target{
		Name:   targetMap["name"].(string),
		Config: configMapString,
	}
}

// canonicalizePolicy sets standarized values for missing fields.
// It must be called after Validate.
func canonicalizePolicy(from *api.ScalingPolicy, to *policy.Policy) {

	if from.Enabled == nil {
		to.Enabled = true
	}

	if evalInterval, ok := from.Policy[keyEvaluationInterval].(string); ok {
		// Ignore parse error since we assume Canonicalize is called after Validate
		to.EvaluationInterval, _ = time.ParseDuration(evalInterval)
	}

	if to.Target.Name == "" {
		to.Target.Name = plugins.InternalTargetNomad
	}

	if to.Target.Config == nil {
		to.Target.Config = make(map[string]string)
	}

	to.Target.Config["job_id"] = from.Target["Job"]
	to.Target.Config["group"] = from.Target["Group"]

	if to.Source == "" {
		to.Source = plugins.InternalAPMNomad
	}

	if to.Source == plugins.InternalAPMNomad {
		to.Query = fmt.Sprintf("%s/%s/%s", to.Query, from.Target["Job"], from.Target["Group"])
	}
}
