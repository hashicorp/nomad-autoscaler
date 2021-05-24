package policy

import (
	"context"
	"fmt"
	hclog "github.com/hashicorp/go-hclog"
)

type MonitorFilterRequest struct {
	ErrCh    chan<- error
	UpdateCh chan<- struct{}
}

// PolicyFilter defines the interface for policy filters used by the
// autoscaler's HA capability.
type PolicyFilter interface {

	// MonitorFilterUpdates accepts a context and a channel for informing the
	// caller of asynchronous updates to the underlying filter.
	MonitorFilterUpdates(ctx context.Context, req MonitorFilterRequest)

	// ReloadFilterMonitor indicates that the filter should be reloaded due to
	// potential changes to configuration and/or clients.
	ReloadFilterMonitor()

	// FilterPolicies should return a list of policies appropriate for this
	// autoscaler agent; policies for other autoscaler agents in the HA pool
	// should be neglected from the returned slice.
	FilterPolicies(policyIDs []PolicyID) []PolicyID
}

// PassThoroughFilter returns the policy source without any extra logic.
func PassThoroughFilter(s Source) Source { return s }

// NewFilteredSource accepts an upstream policy.Source and an ha.PolicyFilter
// and constructs a FilteredSource
func NewFilteredSource(log hclog.Logger, upstreamSource Source, filter PolicyFilter) Source {
	return &FilteredSource{
		log:            log,
		upstreamSource: upstreamSource,
		policyFilter:   filter,
	}
}

// FilteredSource is a policy.Source which accepts policy IDs from an upstream
// policy.Source and filters them through an ha.PolicyFilter
type FilteredSource struct {
	log            hclog.Logger
	upstreamSource Source
	policyFilter   PolicyFilter
}

func (fs *FilteredSource) Source() Source {
	return fs
}

// MonitorIDs calls the same method on the configured upstream policy.Source,
// and filters the discovered policy IDs using the configured PolicyFilter.
func (fs *FilteredSource) MonitorIDs(ctx context.Context, req MonitorIDsReq) {
	if fs.upstreamSource == nil || fs.policyFilter == nil {
		return
	}

	// buffer both of these channels, to prevent the goroutines below from
	// blocking on send between checks of ctx.Done()
	upstreamPolicyCh := make(chan IDMessage, 1)
	filterUpdateCh := make(chan struct{}, 1)

	// create separate error channels just for augmenting the log messages
	policyErrCh := make(chan error, 1)
	filterErrCh := make(chan error, 1)

	go fs.upstreamSource.MonitorIDs(ctx, MonitorIDsReq{
		ErrCh:    policyErrCh,
		ResultCh: upstreamPolicyCh,
	})
	go fs.policyFilter.MonitorFilterUpdates(ctx, MonitorFilterRequest{
		ErrCh:    filterErrCh,
		UpdateCh: filterUpdateCh,
	})

	// keep track of the previous policyIDs, in case the filter updates
	var policyIDs []PolicyID

	// don't emit policy IDs until both the filter and the upstream policy
	// source have sent their first update
	haveFirstPolicies, haveFirstFilter := false, false

	for {
		select {
		case <-ctx.Done():
			fs.log.Trace("stopping file policy source ID monitor")
			return

		case err := <-policyErrCh:
			if err == nil {
				continue
			}
			req.ErrCh <- fmt.Errorf("error from upstream policy source monitor: %v", err)
			continue

		case err := <-filterErrCh:
			if err == nil {
				continue
			}
			req.ErrCh <- fmt.Errorf("error from policy filter monitor: %v", err)
			continue

		case newUpstreamIDs := <-upstreamPolicyCh:
			policyIDs = newUpstreamIDs.IDs
			haveFirstPolicies = true

		case <-filterUpdateCh:
			haveFirstFilter = true
		}

		if !haveFirstPolicies || !haveFirstFilter {
			continue
		}

		newPolicyIDs := fs.policyFilter.FilterPolicies(policyIDs)
		fs.log.Debug("filtered policies", "original_len", len(policyIDs), "filtered_len", len(newPolicyIDs))
		req.ResultCh <- IDMessage{
			IDs:    newPolicyIDs,
			Source: fs.Name(),
		}
	}
}

// MonitorPolicy calls the same method on the configured policy.Source.
// This method doesn't need to worry about the policy filter, because the policy.Manager
// will close the context if the corresponding policy is removed.
func (fs *FilteredSource) MonitorPolicy(ctx context.Context, req MonitorPolicyReq) {
	fs.log.Trace("delegating MonitorPolicy", "policy_id", req.ID)
	fs.upstreamSource.MonitorPolicy(ctx, req)
}

// Name satisfies the Name function of the policy.Source interface.
// There needs to be a match between:
// * the name reported by the policy source during MonitorIDs
// * this method
// * the key for the map of sources in the policy Manager
// * and maybe other stuff as well.
// Therefore, we'll continue to use the existing name.
func (fs *FilteredSource) Name() SourceName {
	return fs.upstreamSource.Name()
}

// ReloadIDsMonitor implements policy.Source by calling the appropriate
// reload method on the underlying policy source and filter.
func (fs *FilteredSource) ReloadIDsMonitor() {
	fs.upstreamSource.ReloadIDsMonitor()
	fs.policyFilter.ReloadFilterMonitor()
}
