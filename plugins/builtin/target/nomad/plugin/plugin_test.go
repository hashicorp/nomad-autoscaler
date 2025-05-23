// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetPlugin_garbageCollect(t *testing.T) {

	curTime := time.Now().UTC().UnixNano()
	testName := "generic GC test"

	// Build the plugin with some populated handlers and data to test.
	targetPlugin := TargetPlugin{
		logger: hclog.NewNullLogger(),
		statusHandlers: map[namespacedJobID]*jobScaleStatusHandler{
			{"default", "running"}:               {jobRunning: true, lastUpdated: curTime},
			{"default", "recently-stopped"}:      {jobRunning: false, lastUpdated: curTime - 1800000000000},
			{"default", "stopped-long-time-ago"}: {jobRunning: false, lastUpdated: curTime - 18000000000000},
			{"special", "running"}:               {jobRunning: true, lastUpdated: curTime},
			{"special", "recently-stopped"}:      {jobRunning: false, lastUpdated: curTime - 1800000000000},
			{"special", "stopped-long-time-ago"}: {jobRunning: false, lastUpdated: curTime - 18000000000000},
		},
	}

	// Trigger the GC.
	targetPlugin.garbageCollect()

	t.Run(testName, func(t *testing.T) {
		assert.Nil(t, targetPlugin.statusHandlers[namespacedJobID{"default", "stopped-long-time-ago"}], testName)
		assert.NotNil(t, targetPlugin.statusHandlers[namespacedJobID{"default", "running"}], testName)
		assert.NotNil(t, targetPlugin.statusHandlers[namespacedJobID{"default", "recently-stopped"}], testName)
		assert.Nil(t, targetPlugin.statusHandlers[namespacedJobID{"special", "stopped-long-time-ago"}], testName)
		assert.NotNil(t, targetPlugin.statusHandlers[namespacedJobID{"special", "running"}], testName)
		assert.NotNil(t, targetPlugin.statusHandlers[namespacedJobID{"special", "recently-stopped"}], testName)
		assert.Len(t, targetPlugin.statusHandlers, 4, testName)
	})
}

func TestTargetPlugin_Status(t *testing.T) {
	nomadMock := httptest.NewServer(http.HandlerFunc(scaleStatusHandler))
	defer nomadMock.Close()

	plugin := PluginConfig.Factory(hclog.NewNullLogger()).(*TargetPlugin)
	plugin.SetConfig(map[string]string{
		"nomad_address": nomadMock.URL,
	})

	expected := &sdk.TargetStatus{
		Ready: true,
		Count: 0,
		Meta: map[string]string{
			"nomad_autoscaler.target.nomad.example.stopped": "false",
		},
	}
	got, err := plugin.Status(map[string]string{
		"Job":       "example",
		"Group":     "cache",
		"Namespace": "default",
	})
	require.NoError(t, err)
	assert.Equal(t, expected, got)

	// Call Status multiple times concurrently to test for data races.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := plugin.Status(map[string]string{
				"Job":       "example",
				"Group":     "cache",
				"Namespace": "default",
			})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

func scaleStatusHandler(w http.ResponseWriter, r *http.Request) {
	respBody := `
{
  "JobCreateIndex": 10,
  "JobID": "example",
  "Namespace": "default",
  "JobModifyIndex": 18,
  "JobStopped": false,
  "TaskGroups": {
    "cache": {
      "Desired": 1,
      "Events": null,
      "Healthy": 1,
      "Placed": 1,
      "Running": 0,
      "Unhealthy": 0
    }
  }
}`
	w.Write([]byte(respBody))
}

func scaleStatusErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}
