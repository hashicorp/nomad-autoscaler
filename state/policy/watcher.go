package policy

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/helper/blocking"
	"github.com/hashicorp/nomad/api"
)

type Watcher struct {
	log             hclog.Logger
	nomad           *api.Client
	lastChangeIndex uint64
	updateChan      chan *api.ScalingPolicy
}

// NewWatcher creates a new scale policies watcher, using blocking queries to
// monitor changes from Nomad.
func NewWatcher(log hclog.Logger, nomad *api.Client, updateChan chan *api.ScalingPolicy) *Watcher {
	return &Watcher{
		log:        log.Named("policy-watcher"),
		nomad:      nomad,
		updateChan: updateChan,
	}
}

// Start triggers the running on the scaling policy watcher, which uses
// blocking queries to monitor the Nomad API for changes in the stored scaling
// polices.
func (w *Watcher) Start() {
	var maxFound uint64

	q := &api.QueryOptions{WaitTime: 5 * time.Minute, WaitIndex: 1}

	for {
		// Perform a blocking query on the Nomad API that returns a stub list
		// of scaling policies. If we get an errors at this point, we should
		// sleep and try again.
		//
		// TODO(jrasell) in the future maybe use a better method than sleep.
		policies, meta, err := w.nomad.Scaling().ListPolicies(q)
		if err != nil {
			w.log.Error("failed to call the Nomad list policies API", "error", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// If the index has not changed, the query returned because the timeout
		// was reached, therefore start the next query loop.
		if !blocking.IndexHasChange(meta.LastIndex, q.WaitIndex) {
			continue
		}

		// Iterate all policies in the list.
		for _, policy := range policies {

			// If the index on the individual policy is not great than our last
			// seen, look at the next policy. If it is great, then move forward
			// and process the policy.
			if !blocking.IndexHasChange(policy.ModifyIndex, w.lastChangeIndex) {
				continue
			}

			// Perform a read on the policy to get all the information.
			p, _, err := w.nomad.Scaling().GetPolicy(policy.ID, nil)
			if err != nil {
				w.log.Error("failed call the Nomad read policy API",
					"error", err, "policy-id", policy.ID)
				continue
			}

			// Send the policy to the channel.
			w.updateChan <- p

			// Update our currently recorded maxFound index.
			maxFound = blocking.FindMaxFound(policy.ModifyIndex, maxFound)
		}

		// Update the Nomad API wait index to start long polling from the
		// correct point and update our recorded lastChangeIndex so we have the
		// correct point to use during the next API return.
		q.WaitIndex = meta.LastIndex
		w.lastChangeIndex = maxFound
	}
}
