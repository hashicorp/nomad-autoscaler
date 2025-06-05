// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ha

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/shoenig/test/must"
)

// TestFilteredSource_MonitorIDs_FilterInput tests that MonitorIDs
// filters the upstream policy IDs
func TestFilteredSource_MonitorIDs_FilterInput(t *testing.T) {

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
	expected := map[policy.PolicyID]policy.PolicyUpdate{
		"abcde": {},
		"a1234": {},
		"aaaaa": {},
	}

	unexpected := map[policy.PolicyID]policy.PolicyUpdate{
		"badbad": {},
		"zzzzzz": {},
		"123456": {},
	}

	allMessages := map[policy.PolicyID]policy.PolicyUpdate{}

	maps.Copy(allMessages, expected)
	maps.Copy(allMessages, unexpected)

	go func() {
		inputCh <- policy.IDMessage{
			IDs:    allMessages,
			Source: "test",
		}
	}()

	// should not receive a message before BOTH filter and upstream have messaged
	select {
	case <-outputCh:
		t.Errorf("received message before filter sent message")
	case <-time.After(2 * time.Second):
	}

	// now set the filter
	testFilter.UpdateFilter(startsWith("a"))

	// check that the policies returned from upstream are filtered
	select {
	case results := <-outputCh:
		must.Eq(t, expected, results.IDs)

	case <-time.After(2 * time.Second):
		t.Errorf("timed out waiting for output message")
	}

	// update the filter, should get new entries
	testFilter.UpdateFilter(startsWith("z"))
	select {
	case results := <-outputCh:
		must.Eq(t, map[policy.PolicyID]policy.PolicyUpdate{"zzzzzz": {}}, results.IDs)
	case <-time.After(2 * time.Second):
		t.Errorf("timed out waiting for output message")
	}

	// check that MonitorIDs returns on context cancel
	monitorCancel()
	select {
	case exited := <-monitorExited:
		//monitor should exit on cancel
		must.True(t, exited)

	case <-time.After(2 * time.Second):
		t.Errorf("timed out waiting for monitor to exit")
	}
}

// TestFilteredSource_MonitorIDs_Errors tests that MonitorIDs
// propagates errors from the underlying policy source and
// filter.
func TestFilteredSource_MonitorIDs_Errors(t *testing.T) {

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
		must.StrContains(t, err.Error(), "error from policy filter monitor: filter_error")
	case <-time.After(2 * time.Second):
		t.Errorf("timed out waiting for filter error")
	}

	// send an error from the upstream source
	go func() {
		upstreamErrCh <- errors.New("source_error")
	}()

	// check that the policies returned from upstream policy source are propagated
	select {
	case err := <-outputErrCh:
		must.StrContains(t, err.Error(), "error from upstream policy source monitor: source_error")
	case <-time.After(2 * time.Second):
		t.Errorf("timed out waiting for source error")
	}
}

// TestFilteredSource_Reload verifies that reload will, at least,
// call the ReloadFilterMonitor() method on the ha.PolicyFilter
func TestFilteredSource_Reload(t *testing.T) {

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
	expected := map[policy.PolicyID]policy.PolicyUpdate{
		"abcde": {},
		"a1234": {},
		"aaaaa": {},
	}

	unexpected := map[policy.PolicyID]policy.PolicyUpdate{
		"badbad": {},
		"zzzzzz": {},
		"123456": {},
	}

	allMessages := map[policy.PolicyID]policy.PolicyUpdate{}

	maps.Copy(allMessages, expected)
	maps.Copy(allMessages, unexpected)

	go func() {
		inputCh <- policy.IDMessage{
			IDs:    allMessages,
			Source: "test",
		}
	}()
	// set the filter
	testFilter.UpdateFilter(startsWith("a"))

	// check that the policies returned from upstream are filtered
	select {
	case results := <-outputCh:
		must.Eq(t, expected, results.IDs)
	case <-time.After(2 * time.Second):
		t.Errorf("timed out waiting for output message")
	}

	// call reload, this should trigger another message send
	source.ReloadIDsMonitor()

	// check that the policies returned from upstream are filtered
	select {
	case results := <-outputCh:
		must.Eq(t, expected, results.IDs)
	case <-time.After(2 * time.Second):
		t.Errorf("timed out waiting for output message")
	}
}
