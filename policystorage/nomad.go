package policystorage

import (
	"fmt"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/plugins"
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

	if fromPolicy.Policy["source"] == nil {
		fromPolicy.Policy["source"] = ""
	}

	target, err := parseTarget(fromPolicy.Policy["target"])
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy target: %v", err)
	}

	toPolicy := &Policy{
		ID:       fromPolicy.ID,
		Min:      *fromPolicy.Min,
		Max:      fromPolicy.Max,
		Source:   fromPolicy.Policy["source"].(string),
		Query:    fromPolicy.Policy["query"].(string),
		Enabled:  *fromPolicy.Enabled,
		Strategy: parseStrategy(fromPolicy.Policy["strategy"]),
		Target:   target,
	}
	canonicalize(fromPolicy, toPolicy)
	return toPolicy, nil
}

func canonicalize(from *api.ScalingPolicy, to *Policy) {

	if from.Enabled == nil {
		to.Enabled = true
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

		parts := strings.Split(to.Query, "_")
		op := parts[0]
		metric := parts[1]

		switch metric {
		case "cpu":
			metric = "nomad.client.allocs.cpu.total_percent"
		case "memory":
			metric = "nomad.client.allocs.memory.usage"
		}

		to.Query = fmt.Sprintf("%s/%s/%s/%s", metric, to.Target.Config["job_id"], to.Target.Config["group"], op)
	}
}

func validate(policy *api.ScalingPolicy) []error {
	var errs []error

	strategyList, ok := policy.Policy["strategy"].([]interface{})
	if !ok {
		errs = append(errs, fmt.Errorf("Policy.strategy (%T) is not a []interface{}", policy.Policy["strategy"]))
		return errs
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
		Config: configMapString,
	}
}

// parseTarget is used to process the target block from within a scaling
// policy.
func parseTarget(target interface{}) (*Target, error) {

	// The Autoscaler policy allows for a non-defined target which means we
	// default to Nomad.
	if target == nil {
		return &Target{}, nil
	}

	targetMap, err := parseGenericBlock(target)
	if err != nil {
		return nil, fmt.Errorf("target %v", err)
	}

	targetCfg, err := parseConfig(targetMap["config"])
	if err != nil {
		return nil, fmt.Errorf("target %v", err)
	}

	var name string

	// Parse the name parameter of the target configuration. If we do not find
	// a map entry the name can be set to an empty string. This means we will
	// default to the Nomad target. If we find the map entry, but are unable to
	// convert this to a string, pass an error back to the caller. This
	// indicates the user has attempted to configure a name, but made a
	// mistake. In this situation we should let them know, rather than apply a
	// default.
	nameInterface, ok := targetMap["name"]
	if ok {
		if name, ok = nameInterface.(string); !ok {
			return nil, fmt.Errorf("target name is %T not string", nameInterface)
		}
	}

	return &Target{
		Name:   name,
		Config: targetCfg,
	}, nil
}

// parseConfig processes the config block from within a scaling target or
// strategy scaling block. It safely unpacks the decoded object, iterating all
// provided config keys.
func parseConfig(config interface{}) (map[string]string, error) {

	// Define our output map.
	cfg := make(map[string]string)

	// Protect against nil config and return quickly.
	if config == nil {
		return cfg, nil
	}

	configMap, err := parseGenericBlock(config)
	if err != nil {
		return nil, fmt.Errorf("config %v", err)
	}

	// Use multierror so we gather all errors from iterating the config map in
	// a single pass.
	var errResult *multierror.Error

	for k, v := range configMap {

		// Assert that the config value is a string. If we are unable to do
		// this add an error to the multierror list and move to the next item.
		stringVal, ok := v.(string)
		if !ok {
			errResult = multierror.Append(errResult,
				fmt.Errorf("config key %s value is %T not string", k, v))
			continue
		}

		// Update the config mapping with our successfully asserted string key
		// and value.
		cfg[k] = stringVal
	}

	// Return the config and any errors. ErrorOrNil() performs a nil check so
	// this is safe to call.
	return cfg, errResult.ErrorOrNil()
}

// parseGenericBlock is a helper function to assert the decoded HCL block type
// and return the map[string]interface{} configured object. The function does
// not protect against nil inputs, this should be done by the caller and
// appropriate action taken there.
func parseGenericBlock(input interface{}) (map[string]interface{}, error) {

	// Validate the input. This is opaque to Nomad on job registration so it is
	// prone to mistakes and misconfiguration.
	listInput, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("block is %T not interface slice", input)
	}

	// HCL decodes the block as a slice of interfaces so check the type on the
	// first item. As described above, this is opaque to Nomad so does not go
	// through validation on job registration.
	mapInput, ok := listInput[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("block first item is %T not map", listInput[0])
	}

	return mapInput, nil
}
