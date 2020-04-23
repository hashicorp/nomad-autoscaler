package policy

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad/api"
)

// Keys represent the scaling policy document keys and help translate
// the opaque object into a usable autoscaling policy.
const (
	keySource             = "source"
	keyQuery              = "query"
	keyEvaluationInterval = "evaluation_interval"
	keyTarget             = "target"
	keyStrategy           = "strategy"
)

const (
	defaultEvaluationInterval = 10 * time.Second
)

// Ensure NomadSource satisfies the Source interface.
var _ Source = (*NomadSource)(nil)

// NomadSource is an implementation of the Source interface that retrieves
// policies from a Nomad cluster.
type NomadSource struct {
	log   hclog.Logger
	nomad *api.Client

	// lock is the mutex that should be used when interacting with the map below.
	lock sync.RWMutex

	// subscribers tracks the current active channels that are listening for
	// policy change events.
	subscribers map[PolicyID]*Subscription
}

// NewNomadSource returns a new Nomad policy source.
func NewNomadSource(log hclog.Logger, nomad *api.Client) *NomadSource {
	return &NomadSource{
		log:         log.Named("nomad_policy_source"),
		nomad:       nomad,
		subscribers: make(map[PolicyID]*Subscription),
	}
}

// Start retrieves a list of policy IDs from a Nomad cluster and send the list
// in the channel when it changes.
//
// This function blocks until ctx is closed.
func (n *NomadSource) Start(ctx context.Context, ch chan<- []PolicyID) {
	n.log.Debug("starting policy blocking query watcher")

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}

	for {
		select {
		case <-ctx.Done():
			n.log.Trace("done")
			return
		default:
			// Perform a blocking query on the Nomad API that returns a stub list
			// of scaling policies. If we get an errors at this point, we should
			// sleep and try again.
			//
			// TODO(jrasell) in the future maybe use a better method than sleep.
			policies, meta, err := n.nomad.Scaling().ListPolicies(q)
			if err != nil {
				n.log.Error("failed to call the Nomad list policies API", "error", err)
				time.Sleep(10 * time.Second)
				continue
			}

			// If the index has not changed, the query returned because the timeout
			// was reached, therefore start the next query loop.
			if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
				continue
			}

			var policyIDs []PolicyID

			// Iterate all policies in the list.
			for _, policy := range policies {
				policyIDs = append(policyIDs, PolicyID(policy.ID))
			}

			// Update the Nomad API wait index to start long polling from the
			// correct point and update our recorded lastChangeIndex so we have the
			// correct point to use during the next API return.
			q.WaitIndex = meta.LastIndex

			// Send new policy IDs in the channel.
			ch <- policyIDs
		}
	}
}

// Subscribe returns a set of channels that can be used by the subscriber to
// receive updates when a policy changes.
//
// This function blocks until the DoneCh in the subscription is closed.
func (n *NomadSource) Subscribe(s *Subscription) {
	log := n.log.With("policy_id", s.ID)

	log.Trace("subscribing to policy")

	policyID := PolicyID(s.ID)

	n.lock.Lock()
	// Close old channel if it already subscribed
	if r, ok := n.subscribers[policyID]; ok {
		n.log.Trace("closing previous subscription channel")
		close(r.DoneCh)
		delete(n.subscribers, policyID)
	}
	n.subscribers[policyID] = s
	n.lock.Unlock()

	log.Trace("subscribed to policy")

	// Blocks until the monitoring function returns
	n.monitorPolicy(s)
}

// monitorPolicy monitors a policy and sends it through the subscription
// channel when a change is detect. Errors are sent in the error channel.
//
// This function blocks until the DoneCh in the subscription is closed.
func (n *NomadSource) monitorPolicy(s *Subscription) {
	log := n.log.With("policy_id", s.ID)

	// Close channels when done with the monitoring loop.
	defer close(s.PolicyCh)
	defer close(s.ErrCh)

	log.Trace("starting policy blocking query watcher")

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}
	for {
		select {
		case <-s.DoneCh:
			return
		default:
			// Perform a blocking query on the Nomad API that returns a stub list
			// of scaling policies. If we get an errors at this point, we should
			// sleep and try again.
			//
			// TODO(jrasell) in the future maybe use a better method than sleep.
			policy, meta, err := n.nomad.Scaling().GetPolicy(string(s.ID), q)
			if err != nil {
				log.Error("failed to call the Nomad get policy API", "error", err)
				s.ErrCh <- fmt.Errorf("failed to get policy: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}

			// If the index has not changed, the query returned because the timeout
			// was reached, therefore start the next query loop.
			if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
				continue
			}

			var autoPolicy Policy
			// TODO(jrasell) once we have a better method for surfacing errors to the
			//  user, this error should be presented.
			if autoPolicy, err = parsePolicy(policy); err != nil {
				log.Error("failed to parse policy", "error", err)
				s.ErrCh <- fmt.Errorf("failed to parse policy: %v", err)
				break
			}

			s.PolicyCh <- autoPolicy
			q.WaitIndex = meta.LastIndex
		}
	}
}

func parsePolicy(p *api.ScalingPolicy) (Policy, error) {
	var to Policy

	if err := validatePolicy(p); err != nil {
		return to, err
	}

	source := p.Policy[keySource]
	if source == nil {
		source = ""
	}

	to = Policy{
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

func validatePolicy(policy *api.ScalingPolicy) error {
	var result error

	evalInterval, ok := policy.Policy[keyEvaluationInterval].(string)
	if ok {
		if _, err := time.ParseDuration(evalInterval); err != nil {
			result = multierror.Append(result, fmt.Errorf("Policy.%s %s is not a time.Durations", keyEvaluationInterval, evalInterval))
		}
	}

	strategyList, ok := policy.Policy[keyStrategy].([]interface{})
	if !ok {
		result = multierror.Append(result, fmt.Errorf("Policy.strategy (%T) is not a []interface{}", policy.Policy[keyStrategy]))
		return result
	}

	_, ok = strategyList[0].(map[string]interface{})
	if !ok {
		result = multierror.Append(result, fmt.Errorf("Policy.strategy[0] (%T) is not a map[string]string", strategyList[0]))
	}

	return result
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

// canonicalizePolicy sets standarized values for missing fields.
// It must be called after Validate.
func canonicalizePolicy(from *api.ScalingPolicy, to *Policy) {

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

		// TODO(luiz) move default query logic handling to the Nomad APM plugin
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
