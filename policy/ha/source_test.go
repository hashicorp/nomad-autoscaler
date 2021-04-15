package ha

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/stretchr/testify/require"
)

// TestFilteredSource_MonitorIDs_FilterInput tests that MonitorIDs
// filters the upstream policy IDs
func TestFilteredSource_MonitorIDs_FilterInput(t *testing.T) {
	require := require.New(t)

	monitorCtx, monitorCancel := context.WithCancel(context.Background())

	inputCh := make(chan policy.IDMessage)
	errCh := make(chan error)
	testFilter := NewTesterFilter(nil)
	testSource := NewTestSource(inputCh, errCh)

	source := NewFilteredSource(hclog.NewNullLogger(), testSource, testFilter)
	outputCh := make(chan policy.IDMessage)
	outputErrCh := make(chan error)
	monitorExited := make(chan bool)
	go func() {
		source.MonitorIDs(monitorCtx, policy.MonitorIDsReq{
			ErrCh:    outputErrCh,
			ResultCh: outputCh,
		})
		monitorExited <- true
	}()

	// send the message from the upstream
	expected := []policy.PolicyID{
		"abcde",
		"a1234",
		"aaaaa",
	}
	unexpected := []policy.PolicyID{
		"badbad",
		"zzzzzz",
		"123456",
	}
	go func() {
		inputCh <- policy.IDMessage{
			IDs:    append(expected, unexpected...),
			Source: "test",
		}
	}()

	// should not receive a message before BOTH filter and upstream have messaged
	select {
	case <-outputCh:
		require.Fail("received message before filter sent message")
	case <-time.After(2 * time.Second):
	}

	// now set the filter
	testFilter.UpdateFilter(startsWith("a"))

	// check that the policies returned from upstream are filtered
	select {
	case results := <-outputCh:
		require.ElementsMatch(expected, results.IDs)
	case <-time.After(2 * time.Second):
		require.Fail("timed out waiting for output message")
	}

	// update the filter, should get new entries
	testFilter.UpdateFilter(startsWith("z"))
	select {
	case results := <-outputCh:
		require.ElementsMatch([]policy.PolicyID{"zzzzzz"}, results.IDs)
	case <-time.After(2 * time.Second):
		require.Fail("timed out waiting for output message")
	}

	// check that MonitorIDs returns on context cancel
	monitorCancel()
	select {
	case exited := <-monitorExited:
		require.True(exited)
	case <-time.After(2 * time.Second):
		require.Fail("timed out waiting for monitor to exit")
	}
}

// TestFilteredSource_MonitorIDs_Errors tests that MonitorIDs
// propagates errors from the underlying policy source and
// filter.
func TestFilteredSource_MonitorIDs_Errors(t *testing.T) {
	require := require.New(t)

	filterErrCh := make(chan error)
	testFilter := NewTesterFilter(filterErrCh)
	upstreamErrCh := make(chan error)
	testSource := NewTestSource(make(chan policy.IDMessage), upstreamErrCh)

	source := NewFilteredSource(hclog.NewNullLogger(), testSource, testFilter)
	outputErrCh := make(chan error)
	go source.MonitorIDs(context.Background(), policy.MonitorIDsReq{
		ErrCh:    outputErrCh,
		ResultCh: make(chan policy.IDMessage),
	})

	// set the filter to anything
	testFilter.UpdateFilter(startsWith(""))

	// send an error from the filter
	go func() {
		filterErrCh <- errors.New("filter_error")
	}()

	// check that the policies returned from policy filter are propagated
	select {
	case err := <-outputErrCh:
		require.EqualError(err, "error from policy filter monitor: filter_error")
	case <-time.After(2 * time.Second):
		require.Fail("timed out waiting for filter error")
	}

	// send an error from the upstream source
	go func() {
		upstreamErrCh <- errors.New("source_error")
	}()

	// check that the policies returned from upstream policy source are propagated
	select {
	case err := <-outputErrCh:
		require.EqualError(err, "error from upstream policy source monitor: source_error")
	case <-time.After(2 * time.Second):
		require.Fail("timed out waiting for source error")
	}
}

// TestFilteredSource_Reload verifies that reload will, at least,
// call the ReloadFilterMonitor() method on the ha.PolicyFilter
func TestFilteredSource_Reload(t *testing.T) {
	require := require.New(t)

	inputCh := make(chan policy.IDMessage)
	errCh := make(chan error)
	testFilter := NewTesterFilter(nil)
	testSource := NewTestSource(inputCh, errCh)

	source := NewFilteredSource(hclog.NewNullLogger(), testSource, testFilter)
	outputCh := make(chan policy.IDMessage)
	outputErrCh := make(chan error)
	go source.MonitorIDs(context.Background(), policy.MonitorIDsReq{
		ErrCh:    outputErrCh,
		ResultCh: outputCh,
	})

	// send the message from the upstream
	expected := []policy.PolicyID{
		"abcde",
		"a1234",
		"aaaaa",
	}
	unexpected := []policy.PolicyID{
		"badbad",
		"zzzzzz",
		"123456",
	}
	go func() {
		inputCh <- policy.IDMessage{
			IDs:    append(expected, unexpected...),
			Source: "test",
		}
	}()
	// set the filter
	testFilter.UpdateFilter(startsWith("a"))

	// check that the policies returned from upstream are filtered
	select {
	case results := <-outputCh:
		require.ElementsMatch(expected, results.IDs)
	case <-time.After(2 * time.Second):
		require.Fail("timed out waiting for output message")
	}

	// call reload, this should trigger another message send
	source.ReloadIDsMonitor()

	// check that the policies returned from upstream are filtered
	select {
	case results := <-outputCh:
		require.ElementsMatch(expected, results.IDs)
	case <-time.After(2 * time.Second):
		require.Fail("timed out waiting for output message")
	}
}
