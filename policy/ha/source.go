// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ha

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// NewFilteredSource accepts an upstream policy.Source and an ha.PolicyFilter
// and constructs a FilteredSource
func NewFilteredSource(log hclog.Logger, upstreamSource policy.Source, filter PolicyFilter) policy.Source {
	return &FilteredSource{
		log:            log,
		upstreamSource: upstreamSource,
		policyFilter:   filter,
		filterCond:     sync.NewCond(&sync.Mutex{}),
	}
}

// FilteredSource is a policy.Source which accepts policy IDs from an upstream
// policy.Source and filters them through an ha.PolicyFilter
type FilteredSource struct {
	log            hclog.Logger
	upstreamSource policy.Source
	policyFilter   PolicyFilter
	filterCond     *sync.Cond
}

// MonitorIDs calls the same method on the configured upstream policy.Source,
// and filters the discovered policy IDs using the configured PolicyFilter.
func (fs *FilteredSource) MonitorIDs(ctx context.Context, req policy.MonitorIDsReq) {
	if fs.upstreamSource == nil || fs.policyFilter == nil {
		return
	}

	// buffer both of these channels, to prevent the goroutines below from
	// blocking on send between checks of ctx.Done()
	upstreamPolicyCh := make(chan policy.IDMessage, 1)
	filterUpdateCh := make(chan struct{}, 1)
	// create separate error channels just for augmenting the log messages
	policyErrCh := make(chan error, 1)
	filterErrCh := make(chan error, 1)

	go fs.upstreamSource.MonitorIDs(ctx, policy.MonitorIDsReq{
		ErrCh:    policyErrCh,
		ResultCh: upstreamPolicyCh,
	})
	go fs.policyFilter.MonitorFilterUpdates(ctx, MonitorFilterRequest{
		ErrCh:    filterErrCh,
		UpdateCh: filterUpdateCh,
	})

	// keep track of the previous policyIDs, in case the filter updates
	var policyIDs map[policy.PolicyID]bool
	// don't emit policy IDs until both the  filter and the upstream policy
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
		fs.log.Trace("filtered policies", "original_len", len(policyIDs), "filtered_len", len(newPolicyIDs))
		req.ResultCh <- policy.IDMessage{
			IDs:    newPolicyIDs,
			Source: fs.Name(),
		}
	}
}

// MonitorPolicy calls the same method on the configured policy.Source.
// This method doesn't need to worry about the policy filter, because the policy.Manager
// will close the context if the corresponding policy is removed.
func (fs *FilteredSource) MonitorPolicy(ctx context.Context, req policy.MonitorPolicyReq) {
	fs.log.Trace("delegating MonitorPolicy", "policy_id", req.ID)
	fs.upstreamSource.MonitorPolicy(ctx, req)
}

// Name satisfies the Name function of the policy.Source interface.
func (fs *FilteredSource) Name() policy.SourceName {
	return policy.SourceNameHA
}

// ReloadIDsMonitor implements policy.Source by calling the appropriate
// reload method on the underlying policy source and filter.
func (fs *FilteredSource) ReloadIDsMonitor() {
	fs.upstreamSource.ReloadIDsMonitor()
	fs.policyFilter.ReloadFilterMonitor()
}

func (fs *FilteredSource) GetLatestVersion(ctx context.Context, pID policy.PolicyID) (*sdk.ScalingPolicy, error) {
	fs.log.Trace("delegating GetPolicy", "policy_id", pID)
	return fs.upstreamSource.GetLatestVersion(ctx, pID)
}
