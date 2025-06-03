// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package policy

import (
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestHandler_calculateRemainingCooldown(t *testing.T) {

	baseTime := time.Now().UTC().UnixNano()

	testCases := []struct {
		inputCooldown  time.Duration
		inputTimestamp int64
		inputLastEvent int64
		expectedOutput time.Duration
		name           string
	}{
		{
			inputCooldown:  20 * time.Minute,
			inputTimestamp: baseTime,
			inputLastEvent: baseTime - 10*time.Minute.Nanoseconds(),
			expectedOutput: 10 * time.Minute,
			name:           "resulting cooldown of 10 minutes",
		},
		{
			inputCooldown:  20 * time.Minute,
			inputTimestamp: baseTime,
			inputLastEvent: baseTime - 25*time.Minute.Nanoseconds(),
			expectedOutput: -5 * time.Minute,
			name:           "negative cooldown period; ie. no cooldown",
		},
	}

	h := NewHandler("", hclog.NewNullLogger(), nil, nil)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput := h.calculateRemainingCooldown(tc.inputCooldown, tc.inputTimestamp, tc.inputLastEvent)
			assert.Equal(t, tc.expectedOutput, actualOutput, tc.name)
		})
	}
}

type MockSource struct {
	resultsCh chan<- sdk.ScalingPolicy
}

func (ms *MockSource) MonitorIDs(ctx context.Context, monitorIDsReq MonitorIDsReq) {}
func (ms *MockSource) MonitorPolicy(ctx context.Context, monitorPolicyReq MonitorPolicyReq) {
	ms.resultsCh = monitorPolicyReq.ResultCh
}

func (ms *MockSource) Name() SourceName {
	return "test"
}
func (ms *MockSource) ReloadIDsMonitor() {}

func TestHandler_ReturnOnPolicyDisabledOrJobStop(t *testing.T) {
	// Set the policy check interval to a high level that will ensure a tick never
	// occurs during this test.

	policyReadInitialTimeout = 10 * time.Minute
	testCases := []struct {
		name string
	}{
		{
			name: "test",
		},
	}

	for _, tc := range testCases {

		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{
				policyID:     "test",
				log:          hclog.NewNullLogger(),
				policySource: &MockSource{},
				ch:           make(chan sdk.ScalingPolicy),
				errCh:        make(chan error),
				cooldownCh:   make(chan time.Duration, 1),
				reloadCh:     make(chan struct{}),
			}

			h.Run(t.Context(), nil)
		})
	}
}
