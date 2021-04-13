package ha

import (
	"context"
	"strings"
	"sync"

	"github.com/hashicorp/nomad-autoscaler/policy"
)

// filterFunc is a simple function to return a list of desired PolicyID
// from a larger list
type filterFunc func(policies []policy.PolicyID) []policy.PolicyID

// testFilter implements ha.PolicyFilter for the purpose of testing.
// It adds a method UpdateFilter which persists the provided
// filterFunc.
type testFilter struct {
	updatedCh chan struct{}
	errCh     chan error

	filter     filterFunc
	filterLock *sync.RWMutex
}

// NewTestFilter returns a testFilter.
// Before using, UpdateFilter must be called.
func NewTesterFilter(errCh chan error) *testFilter {
	return &testFilter{
		updatedCh:  make(chan struct{}),
		errCh:      errCh,
		filter:     nil,
		filterLock: &sync.RWMutex{},
	}
}

// MonitorFilterUpdates fulfills the ha.PolicyFilter interface.
// It returns a message on the provided channel when the filter is updated.
func (f *testFilter) MonitorFilterUpdates(ctx context.Context, req MonitorFilterRequest) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.updatedCh:
			req.UpdateCh <- struct{}{}
		case err := <-f.errCh:
			req.ErrCh <- err
		}
	}
}

// UpdateFilter is a method extended ha.PolicyFilter, used for testing
// filter updates.
func (f *testFilter) UpdateFilter(ff filterFunc) {
	f.filterLock.Lock()
	f.filter = ff
	f.filterLock.Unlock()
	f.updatedCh <- struct{}{}
}

// ReloadFilterMonitor implements ha.PolicyFilter
// For the purpose of testing, this is equivalent to a filter update and
// immediately sends a message to the caller of MonitorFilterUpdates
func (f *testFilter) ReloadFilterMonitor() {
	f.updatedCh <- struct{}{}
}

// FilterPolicies implements ha.PolicyFilter by applying the specified
// filterFunc to the provided policies.
func (f *testFilter) FilterPolicies(policyIDs []policy.PolicyID) []policy.PolicyID {
	f.filterLock.RLock()
	defer f.filterLock.RUnlock()
	if f.filter == nil {
		return policyIDs
	}
	return f.filter(policyIDs)
}

// startsWith is a filterFunc which accepts any PolicyID which starts with the
// configured string.
func startsWith(prefix string) filterFunc {
	return func(input []policy.PolicyID) []policy.PolicyID {
		output := make([]policy.PolicyID, 0)
		for _, pid := range input {
			if strings.HasPrefix(pid.String(), prefix) {
				output = append(output, pid)
			}
		}
		return output
	}
}

// testSource is a test implementation of source.Policy, which simply passes
// messages/errors from input channels to output channels.
type testSource struct {
	inputCh chan policy.IDMessage
	errCh   chan error
}

// NewTestSource returns a policy.Source for testing, which simply echoes
// policy.IDMessage messages from the inputCh to the result channel on
// MonitorIDs
func NewTestSource(inputCh chan policy.IDMessage, errCh chan error) policy.Source {
	return &testSource{
		inputCh: inputCh,
		errCh:   errCh,
	}
}

func (t *testSource) MonitorIDs(ctx context.Context, req policy.MonitorIDsReq) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-t.inputCh:
			req.ResultCh <- msg
		case err := <-t.errCh:
			req.ErrCh <- err
		}
	}
}

func (t *testSource) MonitorPolicy(ctx context.Context, req policy.MonitorPolicyReq) {
	panic("implement me")
}

func (t *testSource) Name() policy.SourceName {
	return "test-source"
}

// ReloadIDsMonitor is a no-op
func (t *testSource) ReloadIDsMonitor() {
	return
}
