package policy

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad/api"
)

type Policy struct {
	ID       string
	Min      int64
	Max      int64
	Source   string
	Query    string
	Enabled  bool
	Target   *Target
	Strategy *Strategy
}

type JobPolicies map[string]*Policy

type Strategy struct {
	Name   string
	Config map[string]string
}

type Target struct {
	Name   string
	Config map[string]string
}

// Keys represent the scaling policy document keys and help translate
// the opaque object into a usable autoscaling policy.
const (
	KeySource   = "source"
	KeyQuery    = "query"
	KeyTarget   = "target"
	KeyStrategy = "strategy"
)

func Canonicalize(from *api.ScalingPolicy, to *Policy) {

	if from.Enabled == nil {
		to.Enabled = true
	}

	if to.Target.Name == "" {
		to.Target.Name = plugins.InternalAPMNomad
	}

	if to.Target.Config == nil {
		to.Target.Config = make(map[string]string)
	}

	to.Target.Config["job_id"] = from.Target["Job"]
	to.Target.Config["group"] = from.Target["Group"]

	if to.Source == "" {
		to.Source = plugins.InternalAPMNomad

		parts := strings.Split(to.Query, "_")
		op := parts[0]
		metric := parts[1]

		switch metric {
		case "cpu":
			metric = "nomad.client.allocs.cpu.total_percent"
		case "memory":
			metric = "nomad.client.allocs.memory.usage"
		}

		to.Query = fmt.Sprintf("%s/%s/%s/%s", metric, from.Target["Job"], from.Target["Group"], op)
	}
}

func Validate(policy *api.ScalingPolicy) []error {
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

func ParseStrategy(s interface{}) *Strategy {
	strategyMap := s.([]interface{})[0].(map[string]interface{})
	configMap := strategyMap["config"].([]interface{})[0].(map[string]interface{})
	configMapString := make(map[string]string)
	for k, v := range configMap {
		configMapString[k] = fmt.Sprintf("%v", v)
	}

	return &Strategy{
		Name:   strategyMap["name"].(string),
		Config: configMapString,
	}
}

func ParseTarget(t interface{}) *Target {
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
