package policystorage

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
)

type Nomad struct {
	Client *api.Client
}

func (n *Nomad) List() ([]*PolicyListStub, error) {
	fromPolicies, _, err := n.Client.Scaling().ListPolicies(nil)
	if err != nil {
		return nil, err
	}

	var toPolicies []*PolicyListStub
	for _, policy := range fromPolicies {
		toPolicy := &PolicyListStub{
			ID: policy.ID,
		}
		toPolicies = append(toPolicies, toPolicy)
	}

	return toPolicies, nil
}

func (n *Nomad) Get(ID string) (*Policy, error) {
	fromPolicy, _, err := n.Client.Scaling().GetPolicy(ID, nil)
	if err != nil {
		return nil, err
	}

	errs := validate(fromPolicy)
	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to parse Policy: %v", errs)
	}

	toPolicy := &Policy{
		ID:       fromPolicy.ID,
		Source:   fromPolicy.Policy["source"].(string),
		Query:    fromPolicy.Policy["query"].(string),
		Strategy: parseStrategy(fromPolicy.Policy["strategy"]),
		Target:   parseTarget(fromPolicy.Policy["target"]),
	}
	canonicalize(fromPolicy, toPolicy)
	return toPolicy, nil
}

func canonicalize(from *api.ScalingPolicy, to *Policy) {
	if to.Target.Name == "" {
		parts := strings.Split(from.Target, "/")
		group := parts[len(parts)-2]

		to.Target.Name = "local-nomad"
		to.Target.Config = map[string]string{
			"job_id": from.JobID,
			"group":  group,
		}
	}
}

func validate(policy *api.ScalingPolicy) []error {
	var errs []error

	strategyList, ok := policy.Policy["strategy"].([]interface{})
	if !ok {
		errs = append(errs, fmt.Errorf("Policy.strategy (%T) is not a []interface{}", policy.Policy["strategy"]))
	}

	_, ok = strategyList[0].(map[string]interface{})
	if !ok {
		errs = append(errs, fmt.Errorf("Policy.strategy[0] (%T) is not a map[string]string", strategyList[0]))
	}

	return errs
}

func parseStrategy(s interface{}) *Strategy {
	strategyMap := s.([]interface{})[0].(map[string]interface{})
	configMap := strategyMap["config"].([]interface{})[0].(map[string]interface{})
	configMapString := make(map[string]string)
	for k, v := range configMap {
		configMapString[k] = fmt.Sprintf("%v", v)
	}

	return &Strategy{
		Name:   strategyMap["name"].(string),
		Min:    int(strategyMap["min"].(float64)),
		Max:    int(strategyMap["max"].(float64)),
		Config: configMapString,
	}
}

func parseTarget(t interface{}) *Target {
	if t == nil {
		return &Target{}
	}

	targetMap := t.([]interface{})[0].(map[string]interface{})
	if targetMap == nil {
		return &Target{}
	}

	var configMapString map[string]string
	if targetMap["config"] != nil {
		configMap := targetMap["config"].([]interface{})[0].(map[string]interface{})
		configMapString = make(map[string]string)
		for k, v := range configMap {
			configMapString[k] = fmt.Sprintf("%v", v)
		}
	}
	return &Target{
		Name:   targetMap["name"].(string),
		Config: configMapString,
	}
}
