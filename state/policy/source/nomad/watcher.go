package nomad

import (
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad-autoscaler/state/policy/source"
	"github.com/hashicorp/nomad/api"
)

// Ensure PolicySource satisfies the source.PolicySource interface.
var _ source.PolicySource = (*PolicySource)(nil)

type PolicySource struct {
	log             hclog.Logger
	nomad           *api.Client
	lastChangeIndex uint64
	reconcileChan   chan []*api.ScalingPolicyListStub
	updateChan      chan *api.ScalingPolicy
}

// NewNomadPolicySource creates a new Nomad scaling policy source which uses
// blocking queries to efficiently track policy updates from the Nomad API.
func NewNomadPolicySource(log hclog.Logger, nomad *api.Client) source.PolicySource {
	return &PolicySource{
		log:   log.Named("policy_source"),
		nomad: nomad,
	}
}

// Start satisfies the Start function on the source.PolicySource interface.
func (ps *PolicySource) Start(updateChan chan *api.ScalingPolicy, reconcileChan chan []*api.ScalingPolicyListStub) {
	ps.log.Debug("starting policy blocking query watcher")

	// Store the update and reconcile channel.
	ps.reconcileChan = reconcileChan
	ps.updateChan = updateChan

	var maxFound uint64

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}

	for {
		// Perform a blocking query on the Nomad API that returns a stub list
		// of scaling policies. If we get an errors at this point, we should
		// sleep and try again.
		//
		// TODO(jrasell) in the future maybe use a better method than sleep.
		policies, meta, err := ps.nomad.Scaling().ListPolicies(q)
		if err != nil {
			ps.log.Error("failed to call the Nomad list policies API", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		ps.reconcile(policies)

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !blocking.IndexHasChanged(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// Iterate all policies in the list.
		for _, policy := range policies {

			// If the index on the individual policy is not great than our last
			// seen, look at the next policy. If it is great, then move forward
			// and process the policy.
			if !blocking.IndexHasChanged(policy.ModifyIndex, ps.lastChangeIndex) {
				continue
			}

			// Perform a read on the policy to get all the information.
			p, _, err := ps.nomad.Scaling().GetPolicy(policy.ID, nil)
			if err != nil {
				ps.log.Error("failed call the Nomad read policy API",
					"error", err, "policy_id", policy.ID)
				continue
			}

			// Send the policy to the channel.
			ps.updateChan <- p

			// Update our currently recorded maxFound index.
			maxFound = blocking.FindMaxFound(policy.ModifyIndex, maxFound)
		}

		// Update the Nomad API wait index to start long polling from the
		// correct point and update our recorded lastChangeIndex so we have the
		// correct point to use during the next API return.
		q.WaitIndex = meta.LastIndex
		ps.lastChangeIndex = maxFound
	}
}

func (ps *PolicySource) reconcile(policies []*api.ScalingPolicyListStub) {
	ps.reconcileChan <- policies
}
