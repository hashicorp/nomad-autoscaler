// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/api"
	"github.com/shoenig/test"
	"google.golang.org/api/compute/v1"
)

func Test_gceNodeIDMap(t *testing.T) {
	testCases := []struct {
		inputNode           *api.Node
		expectedOutputID    string
		expectedOutputError error
		name                string
	}{
		{
			inputNode: &api.Node{
				Attributes: map[string]string{
					"platform.gce.zone":            "us-central1-f",
					"unique.platform.gce.hostname": "instance-1.c.project.internal",
				},
			},
			expectedOutputID:    "zones/us-central1-f/instances/instance-1",
			expectedOutputError: nil,
			name:                "required attributes found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{
					"platform.gce.zone":            "us-central1-f",
					"unique.platform.gce.hostname": "instance-1",
				},
			},
			expectedOutputID:    "zones/us-central1-f/instances/instance-1",
			expectedOutputError: nil,
			name:                "required attributes found with non-split hostname",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "platform.gce.zone" not found`),
			name:                "required attribute zone not found",
		},
		{
			inputNode: &api.Node{
				Attributes: map[string]string{
					"platform.gce.zone": "us-central1-f",
				},
			},
			expectedOutputID:    "",
			expectedOutputError: errors.New(`attribute "unique.platform.gce.hostname" not found`),
			name:                "required attribute hostname not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualID, actualErr := gceNodeIDMap(tc.inputNode)
			test.Eq(t, tc.expectedOutputID, actualID, test.Sprint("IDs should be equal"))

			if tc.expectedOutputError != nil {
				test.EqError(t, tc.expectedOutputError, actualErr.Error())
			} else {
				test.NoError(t, actualErr, test.Sprint("expected no error"))
			}
		})
	}
}

// mockInstanceGroup implements the instanceGroup interface for the purpose of testing.
type mockInstanceGroup struct {
	name              string
	statusFunc        func(context.Context, *compute.Service) (bool, int64, error)
	resizeTarget      int64
	noCreationRetries bool
}

func (m *mockInstanceGroup) getName() string {
	return m.name
}

func (m *mockInstanceGroup) status(ctx context.Context, s *compute.Service) (bool, int64, error) {
	return m.statusFunc(ctx, s)
}

func (m *mockInstanceGroup) resize(ctx context.Context, s *compute.Service, c *http.Client, num int64, noCreationRetries bool) error {
	m.resizeTarget = num
	m.noCreationRetries = noCreationRetries
	return nil
}

func (m *mockInstanceGroup) deleteInstance(ctx context.Context, s *compute.Service, instances []string) error {
	return nil
}

func (m *mockInstanceGroup) listInstances(ctx context.Context, s *compute.Service) ([]*compute.ManagedInstance, error) {
	return nil, nil
}

func TestTargetPlugin_ensureInstanceGroupIsStable(t *testing.T) {
	// Test case 1: MIG becomes stable within the retry limit.
	t.Run("mig becomes stable", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 3

		attempts := 0
		mockIG := &mockInstanceGroup{
			name: "test-mig-success",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				attempts++
				if attempts <= 2 {
					return false, 0, nil
				}
				return true, 10, nil
			},
		}

		err := tp.ensureInstanceGroupIsStable(context.Background(), mockIG, 15)

		test.NoError(t, err, test.Sprint("expected no error when MIG becomes stable"))
		test.Eq(t, 3, attempts, test.Sprint("expected 3 attempts to become stable"))
	})

	// Test case 2: MIG never becomes stable and reaches the retry limit.
	t.Run("mig never becomes stable", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 2

		attempts := 0
		mockIG := &mockInstanceGroup{
			name: "test-mig-failure",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				attempts++
				return false, 0, nil
			},
		}

		err := tp.ensureInstanceGroupIsStable(context.Background(), mockIG, 2)

		test.ErrorContains(t, err, "reached retry limit", test.Sprint("expected an error with the correct message"))
		test.Eq(t, 2, attempts, test.Sprint("expected 2 attempts (the limit) to be made"))
	})

	// Test case 3: Custom retry attempts parameter.
	t.Run("custom retry attempts", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 2

		attempts := 0
		mockIG := &mockInstanceGroup{
			name: "test-mig-override",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				attempts++
				if attempts <= 3 {
					return false, 0, nil
				}
				return true, 10, nil
			},
		}

		// Test with 4 retry attempts (custom value)
		err := tp.ensureInstanceGroupIsStable(context.Background(), mockIG, 4)

		test.NoError(t, err, test.Sprint("expected no error when MIG becomes stable with custom retry attempts"))
		test.Eq(t, 4, attempts, test.Sprint("expected 4 attempts based on custom retry attempts"))
	})
}

func TestTargetPlugin_resolveRetryAttempts(t *testing.T) {
	t.Run("uses plugin default when no config override", func(t *testing.T) {
		tp := &TargetPlugin{retryAttempts: 15}
		config := map[string]string{}

		result, err := tp.resolveRetryAttempts(config)

		test.NoError(t, err)
		test.Eq(t, 15, result, test.Sprint("should use plugin default"))
	})

	t.Run("uses policy override when provided", func(t *testing.T) {
		tp := &TargetPlugin{retryAttempts: 15}
		config := map[string]string{
			"retry_attempts": "30",
		}

		result, err := tp.resolveRetryAttempts(config)

		test.NoError(t, err)
		test.Eq(t, 30, result, test.Sprint("should use policy override"))
	})

	t.Run("returns error for invalid retry_attempts", func(t *testing.T) {
		tp := &TargetPlugin{retryAttempts: 15}
		config := map[string]string{
			"retry_attempts": "invalid",
		}

		result, err := tp.resolveRetryAttempts(config)

		test.Error(t, err)
		test.Eq(t, 0, result, test.Sprint("should return 0 on error"))
		test.StrContains(t, err.Error(), "failed to parse retry_attempts value from policy")
	})
}

func TestTargetPlugin_resolveNoCreationRetries(t *testing.T) {
	t.Run("uses plugin default when no config override", func(t *testing.T) {
		tp := &TargetPlugin{noCreationRetries: true}
		config := map[string]string{}

		result, err := tp.resolveNoCreationRetries(config)

		test.NoError(t, err)
		test.True(t, result)
	})

	t.Run("uses policy override when provided", func(t *testing.T) {
		tp := &TargetPlugin{noCreationRetries: false}
		config := map[string]string{
			configKeyNoCreationRetries: "true",
		}

		result, err := tp.resolveNoCreationRetries(config)

		test.NoError(t, err)
		test.True(t, result)
	})

	t.Run("returns error for invalid no_creation_retries", func(t *testing.T) {
		tp := &TargetPlugin{noCreationRetries: false}
		config := map[string]string{
			configKeyNoCreationRetries: "invalid",
		}

		result, err := tp.resolveNoCreationRetries(config)

		test.Error(t, err)
		test.False(t, result)
		test.StrContains(t, err.Error(), "failed to parse no_creation_retries value from policy")
	})
}

func TestTargetPlugin_ensureScaleOutComplete(t *testing.T) {
	t.Run("skips target size check when no creation retries disabled", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 1

		mockIG := &mockInstanceGroup{
			name: "test-mig",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				return true, 3, nil
			},
		}

		err := tp.ensureScaleOutComplete(context.Background(), mockIG, 5, map[string]string{}, false)
		test.NoError(t, err)
	})

	t.Run("fails when no creation retries enabled and target shortfall remains", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 1

		mockIG := &mockInstanceGroup{
			name: "test-mig",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				return true, 3, nil
			},
		}

		err := tp.ensureScaleOutComplete(context.Background(), mockIG, 5, map[string]string{}, true)
		test.ErrorContains(t, err, "instance group stabilized at 3 instances; expected 5")
	})
}

func TestTargetPlugin_scaleOut(t *testing.T) {
	t.Run("passes no creation retries to resize path", func(t *testing.T) {
		tp := NewGCEMIGPlugin(hclog.NewNullLogger())
		tp.retryAttempts = 1
		tp.noCreationRetries = true

		mockIG := &mockInstanceGroup{
			name: "test-mig",
			statusFunc: func(ctx context.Context, s *compute.Service) (bool, int64, error) {
				return true, 7, nil
			},
		}

		err := tp.scaleOut(context.Background(), mockIG, 7, map[string]string{})

		test.NoError(t, err)
		test.Eq(t, int64(7), mockIG.resizeTarget)
		test.True(t, mockIG.noCreationRetries)
	})
}

func Test_resizeAdvanced(t *testing.T) {
	t.Run("sends expected request body", func(t *testing.T) {
		var gotMethod, gotPath, gotContentType string
		var gotTargetSize int64
		var gotNoCreationRetries bool

		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotMethod = req.Method
				gotPath = req.URL.String()
				gotContentType = req.Header.Get("Content-Type")

				var body resizeAdvancedRequest
				test.NoError(t, json.NewDecoder(req.Body).Decode(&body))
				gotTargetSize = body.TargetSize
				gotNoCreationRetries = body.NoCreationRetries

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       http.NoBody,
					Header:     make(http.Header),
					Request:    req,
				}, nil
			}),
		}

		err := resizeAdvanced(context.Background(), client, "https://example.com/resizeAdvanced", resizeAdvancedRequest{
			TargetSize:        9,
			NoCreationRetries: true,
		})

		test.NoError(t, err)
		test.Eq(t, http.MethodPost, gotMethod)
		test.Eq(t, "https://example.com/resizeAdvanced", gotPath)
		test.Eq(t, "application/json", gotContentType)
		test.Eq(t, int64(9), gotTargetSize)
		test.True(t, gotNoCreationRetries)
	})

	t.Run("returns request errors", func(t *testing.T) {
		client := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("boom")
			}),
		}

		err := resizeAdvanced(context.Background(), client, "https://example.com/resizeAdvanced", resizeAdvancedRequest{})
		test.ErrorContains(t, err, "boom")
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
