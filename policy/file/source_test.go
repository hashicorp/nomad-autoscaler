// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package file

import (
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/policy"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
)

func TestSource_getFilePolicyID(t *testing.T) {
	testCases := []struct {
		inputFile       string
		inputPolicyName string
		existingID      policy.PolicyID
		inputSource     *Source
		name            string
	}{
		{
			inputFile:       "/this/test/file.hcl",
			inputPolicyName: "policy_name",
			existingID:      "b65aa225-35bd-aa72-29d0-a1d228662817",
			inputSource: &Source{idMap: map[pathMD5Sum]policy.PolicyID{
				md5Sum("/this/test/file.hcl/policy_name"): "b65aa225-35bd-aa72-29d0-a1d228662817",
			}},
			name: "file already within idMap",
		},

		{
			inputFile:       "/this/test/file.hcl",
			inputPolicyName: "policy_name",
			existingID:      "",
			inputSource:     &Source{idMap: map[pathMD5Sum]policy.PolicyID{}},
			name:            "file not within idMap",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outputID := tc.inputSource.getFilePolicyID(tc.inputFile, tc.inputPolicyName)

			if tc.existingID != "" {
				assert.Equal(t, tc.existingID, outputID, tc.name)
			}

			policyID, ok := tc.inputSource.idMap[md5Sum(tc.inputFile+"/"+tc.inputPolicyName)]
			assert.Equal(t, policyID, outputID, tc.name)
			assert.True(t, ok, tc.name)
		})
	}
}

// testFileSource reads the policies from the given directory
// and returns a Source and one policyID from that directory.
func testFileSource(t *testing.T, dir string) (*Source, policy.PolicyID) {
	t.Helper()
	src := NewFileSource(
		hclog.Default(),
		dir, // should contain real policy files.
		policy.NewProcessor(
			&policy.ConfigDefaults{
				DefaultEvaluationInterval: time.Second,
				DefaultCooldown:           time.Second},
			[]string{},
		),
	)
	s := src.(*Source)

	idsMap, err := s.handleDir() // populate idMap and policyMap
	if err != nil {
		t.Fatalf("error from handleDir: %v", err)
	}
	if len(idsMap) == 0 {
		t.Fatalf("uh oh, no policies in %v", dir)
	}

	id := ""
	for k := range idsMap {
		id = k
		break // just take the first one

	}

	return s, id
}

func TestSource_MonitorPolicy(t *testing.T) {
	s, pid := testFileSource(t, "./test-fixtures")

	// running MonitorPolicy() twice should have the same result.
	for _, n := range []string{"round one", "round two"} {

		// not using sub-tests here, because if round one fails,
		// there's no sense running round two.
		// this little helper gives context for any failures.
		fatal := func(msg string, args ...any) {
			t.Helper()
			t.Fatalf(n+": "+msg, args...)
		}

		errCh := make(chan error)
		resultCh := make(chan sdk.ScalingPolicy)
		reloadCh := make(chan struct{}, 1)
		req := policy.MonitorPolicyReq{
			ID:       pid,
			ErrCh:    errCh,
			ResultCh: resultCh,
			ReloadCh: reloadCh,
		}

		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)

		// method under test
		go s.MonitorPolicy(ctx, req)

		// expect initial policy
		select {
		case <-time.After(time.Second): // plenty long enough
			fatal("failed to receive policy in time")
		case err := <-errCh:
			fatal("error in test: %s", err)
		case i := <-resultCh:
			t.Logf("good show, got initial policy: %+v", i)
		}

		// test reload with no changes
		select {
		case reloadCh <- struct{}{}: // cause reload
		case <-time.After(time.Millisecond * 10):
			fatal("unable to write to reloadCh")
		}
		select {
		case <-time.After(time.Millisecond * 100):
			t.Log("good show, not expecting any updates")
		case err := <-errCh:
			fatal("error in reload test: %s", err)
		case i := <-resultCh:
			fatal("did not expect a result from no-op reload, got: %v", i)
		}

		// modify the policy, then test reload again. should get changes.
		s.policyMapLock.Lock()
		s.policyMap[pid].policy.Max = 99
		s.policyMapLock.Unlock()
		select {
		case reloadCh <- struct{}{}: // cause reload
		case <-time.After(time.Millisecond * 10):
			fatal("unable to write to reloadCh")
		}
		select {
		case <-time.After(time.Second):
			fatal("should have received updated policy")
		case err := <-errCh:
			fatal("error in second reload test: %s", err)
		case i := <-resultCh:
			t.Logf("good show, got update from disk: %+v", i)
			// it should be set back to what's on disk.
			if i.Max == 99 {
				fatal("max should not still be test value 99")
			}
		}

		cancel()
	}
}

func TestSource_MonitorPolicy_ContinueOnError(t *testing.T) {
	// start with a happy source
	src, pid := testFileSource(t, "./test-fixtures")

	// then break it
	src.policyMap[pid].file = "/nowhere"

	errCh := make(chan error)
	resultCh := make(chan sdk.ScalingPolicy)
	reloadCh := make(chan struct{}, 1)
	req := policy.MonitorPolicyReq{
		ID:       pid,
		ErrCh:    errCh,
		ResultCh: resultCh,
		ReloadCh: reloadCh,
	}

	// done chan separate from ctx to ensure the goroutine completes
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		src.MonitorPolicy(ctx, req)

		select {
		case <-ctx.Done():
			t.Logf("good, context done")
		default:
			t.Error("MonitorPolicy should only return if the context is done")
		}

		close(done)
	}()

	select {
	case <-time.After(time.Millisecond * 200):
		t.Fatal("no error in time")
	case err := <-errCh:
		t.Logf("good show, got error as expected: %v", err)
	case i := <-resultCh:
		t.Fatalf("not expecting a policy, got: %+v", i)
	}

	cancel()
	<-done
}
