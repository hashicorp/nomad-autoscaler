// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package scaleutils

import (
	"context"
	"errors"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	multierror "github.com/hashicorp/go-multierror"
	errHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/error"
	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
)

// Mock implementation of the drainer, it includes a couple of callback functions
// that can be used to assert the correctness of the call values.
type mockDrainer struct {
	drainerMockFunc func(nodeID string, opts *api.DrainOptions, q *api.WriteOptions) (*api.NodeDrainUpdateResponse, error)
	monitorMockFunc func(ctx context.Context, nodeID string, index uint64, ignoreSys bool) <-chan *api.MonitorMessage

	monitorFunctionCalled bool
}

func newMockDrainer() *mockDrainer {
	return &mockDrainer{
		monitorFunctionCalled: false,
	}
}

func (md *mockDrainer) UpdateDrainOpts(nodeID string, opt *api.DrainOptions, q *api.WriteOptions) (*api.NodeDrainUpdateResponse, error) {
	return md.drainerMockFunc(nodeID, opt, q)
}

func (md *mockDrainer) MonitorDrain(ctx context.Context, nodeID string, index uint64, ignoreSys bool) <-chan *api.MonitorMessage {
	md.monitorFunctionCalled = true

	return md.monitorMockFunc(ctx, nodeID, index, ignoreSys)
}

func TestNewClusterScaleUtils_drainSpec(t *testing.T) {
	testCases := []struct {
		inputCfg            map[string]string
		expectedOutputSpec  *api.DrainSpec
		expectedOutputError *multierror.Error
		name                string
	}{
		{
			inputCfg: map[string]string{},
			expectedOutputSpec: &api.DrainSpec{
				Deadline:         15 * time.Minute,
				IgnoreSystemJobs: false,
			},
			expectedOutputError: nil,
			name:                "no user parameters set",
		},
		{
			inputCfg: map[string]string{
				"node_drain_deadline":           "10m",
				"node_drain_ignore_system_jobs": "true",
			},
			expectedOutputSpec: &api.DrainSpec{
				Deadline:         10 * time.Minute,
				IgnoreSystemJobs: true,
			},
			expectedOutputError: nil,
			name:                "all parameters set in config",
		},
		{
			inputCfg: map[string]string{
				"node_drain_deadline": "10mm",
			},
			expectedOutputSpec: nil,
			expectedOutputError: &multierror.Error{
				Errors:      []error{errors.New(`time: unknown unit "mm" in duration "10mm"`)},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "config deadline parse error",
		},
		{
			inputCfg: map[string]string{
				"node_drain_ignore_system_jobs": "maybe",
			},
			expectedOutputSpec: nil,
			expectedOutputError: &multierror.Error{
				Errors:      []error{errors.New(`strconv.ParseBool: parsing "maybe": invalid syntax`)},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "config ignore system jobs parse error",
		},
		{
			inputCfg: map[string]string{
				"node_drain_deadline":           "10mm",
				"node_drain_ignore_system_jobs": "maybe",
			},
			expectedOutputSpec: nil,
			expectedOutputError: &multierror.Error{
				Errors: []error{
					errors.New(`time: unknown unit "mm" in duration "10mm"`),
					errors.New(`strconv.ParseBool: parsing "maybe": invalid syntax`),
				},
				ErrorFormat: errHelper.MultiErrorFunc,
			},
			name: "multi config params parse error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualDrainSpec, actualError := drainSpec(tc.inputCfg)
			assert.Equal(t, tc.expectedOutputSpec, actualDrainSpec, tc.name)
			if tc.expectedOutputError != nil {
				assert.EqualError(t, tc.expectedOutputError, actualError.Error(), tc.name)
			}
		})
	}
}

func Test_DrainNode(t *testing.T) {
	testNodeID := "nodeID"
	testLogger := hclog.New(&hclog.LoggerOptions{
		Level: hclog.LevelFromString("ERROR"),
	})

	md := newMockDrainer()

	md.drainerMockFunc = func(nodeID string, opts *api.DrainOptions, _ *api.WriteOptions) (*api.NodeDrainUpdateResponse, error) {
		must.StrContains(t, testNodeID, nodeID)
		must.StrContains(t, opts.Meta[NODE_DRAINED_META_KEY], NODE_DRAINED_META_VALUE)

		return &api.NodeDrainUpdateResponse{}, nil
	}

	md.monitorMockFunc = func(ctx context.Context, nodeID string, index uint64, ignoreSys bool) <-chan *api.MonitorMessage {
		must.StrContains(t, testNodeID, nodeID)

		outCh := make(chan *api.MonitorMessage, 1)
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(outCh)
		}()

		return outCh
	}

	cu := &ClusterScaleUtils{
		log:     testLogger,
		drainer: md,
	}

	ctx := context.Background()
	err := cu.drainNode(ctx, testNodeID, &api.DrainSpec{})
	must.NoError(t, err)
	must.True(t, md.monitorFunctionCalled)

}
