package policy

import (
	"context"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/manager"
	targetpkg "github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/kr/pretty"
)

// Handler monitors a policy for changes and controls when them are sent for
// evaluation.
type Handler struct {
	log hclog.Logger

	// policyID is the ID of the policy the handler is responsible for.
	policyID PolicyID

	// pluginManager is used to retrieve an instance of the target plugin used
	// by the policy.
	pluginManager *manager.PluginManager

	// policySource is used to monitor for changes to the policy the handler
	// is responsible for.
	policySource Source

	// ticker controls the frequency the policy is sent for evaluation.
	ticker *time.Ticker

	// running is used to help keep track if the handler is active or not.
	running     bool
	runningLock sync.RWMutex

	// ch is used to listen for policy updates.
	ch chan Policy

	// errCh is used to listen for errors from the policy source.
	errCh chan error

	// doneCh is used to signal the handler to stop.
	doneCh chan struct{}
}

// NewHandler returns a new handler for a policy.
func NewHandler(ID PolicyID, log hclog.Logger, pm *manager.PluginManager, ps Source) *Handler {
	return &Handler{
		policyID:      ID,
		log:           log.Named("policy_handler").With("policy_id", ID),
		pluginManager: pm,
		policySource:  ps,
		ch:            make(chan Policy),
		errCh:         make(chan error),
		doneCh:        make(chan struct{}),
	}
}

// Run starts the handler and periodically sends the policy for evaluation.
//
// This function blocks until the context provided is canceled or the handler
// is stopped with the Stop() method.
func (h *Handler) Run(ctx context.Context, evalCh chan<- *Evaluation) {
	h.log.Trace("starting policy handler")

	defer h.Stop()

	// Mark the handler as running.
	h.runningLock.Lock()
	h.running = true
	h.runningLock.Unlock()

	// Store a local copy of the policy so we can compare it for changes.
	var policy *Policy

	// Start with a long ticker until we receive the right interval.
	// TODO(luiz): make this a config param
	policyReadTimeout := 3 * time.Minute
	h.ticker = time.NewTicker(policyReadTimeout)

	// Start monitoring the policy for changes.
	go h.policySource.MonitorPolicy(ctx, h.policyID, h.ch, h.errCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.doneCh:
			return
		case err := <-h.errCh:
			// TODO(luiz) add better error handling
			h.log.Error(err.Error())
		case p := <-h.ch:
			h.log.Trace("policy changed")
			h.log.Trace(pretty.Sprint(p))

			if policy == nil {
				// We received the proper policy value, we can now set the
				// ticker with the right interval.
				h.ticker.Stop()
				h.ticker = time.NewTicker(p.EvaluationInterval)
			}

			// TODO(luiz): handle policy update

			// Update local copy of the policy
			policy = &p
		case <-h.ticker.C:
			h.log.Trace("tick")

			if policy == nil {
				// Initial ticker ticked without a policy being set, assume we are not able
				// to retrieve the policy and exit.
				h.log.Error("timeout: failed to read policy in time")
				return
			}

			// Exit early if the policy is not enabled.
			if !policy.Enabled {
				h.log.Trace("policy is not enabled")
				continue
			}

			// Dispense an instance of taret plugin used by the policy.
			targetPlugin, err := h.pluginManager.Dispense(policy.Target.Name, plugins.PluginTypeTarget)
			if err != nil {
				h.log.Error("failed to dispense plugin", "error", err)
				continue
			}
			targetInst, ok := targetPlugin.Plugin().(targetpkg.Target)
			if !ok {
				h.log.Error("target plugin is not a taret plugin")
				return
			}

			// Get target status.
			h.log.Trace("getting target status")

			status, err := targetInst.Status(policy.Target.Config)
			if err != nil {
				h.log.Error("failed to get target status", "error", err)
				continue
			}

			// A nil status indicates the target doesn't exist, so we don't need to
			// monitor the policy anymore.
			//
			// TODO(luiz): if the policy is not removed the handler will keep being
			// destroyed and recreated. Maybe wait instead of return?
			if status == nil {
				h.log.Trace("target doesn't exist anymore", "target", policy.Target.Config)
				return
			}

			// Exit early if the target is not ready yet.
			if !status.Ready {
				h.log.Trace("target is not ready")
				continue
			}

			// Send policy for evaluation.
			h.log.Trace("sending policy for evaluation")
			evalCh <- &Evaluation{
				Policy:       policy,
				TargetStatus: status,
			}
		}
	}
}

// Stop stops the handler and the monitoring Go routine.
func (h *Handler) Stop() {
	h.runningLock.Lock()
	defer h.runningLock.Unlock()

	if h.running {
		h.log.Trace("stopping handler")
		h.ticker.Stop()
		close(h.doneCh)
	}

	h.running = false
}
