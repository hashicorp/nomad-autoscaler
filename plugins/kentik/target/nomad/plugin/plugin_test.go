// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
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
			{"default", "running"}:               {isRunning: true, lastUpdated: curTime},
			{"default", "recently-stopped"}:      {isRunning: false, lastUpdated: curTime - 1800000000000},
			{"default", "stopped-long-time-ago"}: {isRunning: false, lastUpdated: curTime - 18000000000000},
			{"special", "running"}:               {isRunning: true, lastUpdated: curTime},
			{"special", "recently-stopped"}:      {isRunning: false, lastUpdated: curTime - 1800000000000},
			{"special", "stopped-long-time-ago"}: {isRunning: false, lastUpdated: curTime - 18000000000000},
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
	nomadMock := httptest.NewServer(http.HandlerFunc(statusHandler(false, false)))
	defer nomadMock.Close()

	plugin := PluginConfig.Factory(hclog.NewNullLogger()).(*TargetPlugin)
	plugin.SetConfig(map[string]string{
		"nomad_address": nomadMock.URL,
	})

	expected := &sdk.TargetStatus{
		Ready: true,
		Count: 1,
		Meta: map[string]string{
			"nomad_autoscaler.target.nomad.example.stopped": "false",
		},
	}
	got, err := plugin.Status(map[string]string{
		"Job":                "example",
		"Group":              "cache",
		"Namespace":          "default",
		"CheckUnknownAllocs": "false",
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

func TestTargetPlugin_Status_UnknownAllocs(t *testing.T) {
	nomadMock := httptest.NewServer(http.HandlerFunc(statusHandler(true, false)))
	defer nomadMock.Close()

	plugin := PluginConfig.Factory(hclog.NewNullLogger()).(*TargetPlugin)
	plugin.SetConfig(map[string]string{
		"nomad_address": nomadMock.URL,
	})

	got, err := plugin.Status(map[string]string{
		"Job":                "example",
		"Group":              "cache",
		"Namespace":          "default",
		"CheckUnknownAllocs": "true",
	})

	assert.Nil(t, got)
	assert.Error(t, err)

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
			assert.Error(t, err)
		}()
	}
	wg.Wait()
}

func TestTargetPlugin_Status_UnknownAllocsWithIneligibleNode(t *testing.T) {
	nomadMock := httptest.NewServer(http.HandlerFunc(statusHandler(true, true)))
	defer nomadMock.Close()

	plugin := PluginConfig.Factory(hclog.NewNullLogger()).(*TargetPlugin)
	plugin.SetConfig(map[string]string{
		"nomad_address": nomadMock.URL,
	})

	expected := &sdk.TargetStatus{
		Ready: true,
		Count: 2, // should count the ineligible node
		Meta: map[string]string{
			"nomad_autoscaler.target.nomad.example.stopped": "false",
		},
	}

	got, err := plugin.Status(map[string]string{
		"Job":                "example",
		"Group":              "cache",
		"Namespace":          "default",
		"CheckUnknownAllocs": "true",
	})

	assert.Equal(t, expected, got)
	assert.Nil(t, err)

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
			assert.Nil(t, err)
		}()
	}
	wg.Wait()
}

func TestTargetPlugin_statusTimeout(t *testing.T) {
	nomadMock := httptest.NewServer(http.HandlerFunc(scaleStatusErrorHandler))
	defer nomadMock.Close()

	plugin := PluginConfig.Factory(hclog.NewNullLogger()).(*TargetPlugin)
	plugin.SetConfig(map[string]string{
		"nomad_address": nomadMock.URL,
	})

	var statusErr error
	var status *sdk.TargetStatus
	doneCh := make(chan struct{})
	go func() {
		status, statusErr = plugin.Status(map[string]string{
			"Job":       "example",
			"Group":     "cache",
			"Namespace": "default",
		})
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-time.After(2 * statusHandlerInitTimeout):
		t.Fatalf("status call blocked")
	}

	assert.Error(t, statusErr)
	assert.Nil(t, status)
}

func statusHandler(unknownAllocs, ineligibleNodes bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/scale") {
			scaleStatusHandler(w, r)
			return
		} else if strings.HasSuffix(r.URL.Path, "/allocations") {
			allocsHandler(unknownAllocs)(w, r)
			return
		} else if strings.HasSuffix(r.URL.Path, "/nodes") {
			nodesHandler(ineligibleNodes)(w, r)
			return
		}
	}
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
      "Running": 1,
      "Unhealthy": 0
    }
  }
}`
	w.Write([]byte(respBody))
}

func allocsHandler(unknownAllocs bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		state := "running"
		if unknownAllocs {
			state = "unknown"
		}
		respBody := fmt.Sprintf(`
[
  {
    "ID": "ed344e0a-7290-d117-41d3-a64f853ca3c2",
    "EvalID": "a9c5effc-2242-51b2-f1fe-054ee11ab189",
    "Name": "example.cache[0]",
    "NodeID": "cb1f6030-a220-4f92-57dc-7baaabdc3823",
    "PreviousAllocation": "516d2753-0513-cfc7-57ac-2d6fac18b9dc",
    "NextAllocation": "cd13d9b9-4f97-7184-c88b-7b451981616b",
    "JobID": "example",
    "TaskGroup": "cache",
    "DesiredStatus": "run",
    "DesiredDescription": "",
    "ClientStatus": "%s",
    "ClientDescription": "",
    "TaskStates": {
      "redis": {
        "State": "%s",
        "Failed": false,
        "StartedAt": "2017-05-25T23:41:23.240184101Z",
        "FinishedAt": "0001-01-01T00:00:00Z"
      }
    },
    "CreateIndex": 9,
    "ModifyIndex": 13,
    "CreateTime": 1495755675944527600,
    "ModifyTime": 1495755675944527600
  }
]`, state, state)
		w.Write([]byte(respBody))
	}
}

func nodesHandler(ineligibleNodes bool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "eligible"
		if ineligibleNodes {
			status = "ineligible"
		}

		respBody := fmt.Sprintf(`
[
  {
    "Address": "10.138.0.5",
    "Attributes": {
      "os.name": "ubuntu"
    },
    "CreateIndex": 6,
    "Datacenter": "dc1",
    "Drain": false,
    "Drivers": {
      "docker": {
        "Attributes": {
          "driver.docker.bridge_ip": "172.17.0.1",
          "driver.docker.version": "18.03.0-ce",
          "driver.docker.volumes.enabled": "1"
        },
        "Detected": true,
        "HealthDescription": "Driver is available and responsive",
        "Healthy": true,
        "UpdateTime": "2018-04-11T23:34:48.713720323Z"
      }
    },
    "ID": "cb1f6030-a220-4f92-57dc-7baaabdc3823",
    "LastDrain": null,
    "ModifyIndex": 2526,
    "Name": "nomad-4",
    "NodeClass": "",
    "SchedulingEligibility": "%s",
    "Status": "ready",
    "StatusDescription": "",
    "Version": "0.8.0-rc1"
  }
]`, status)
		_, _ = w.Write([]byte(respBody))
	}
}

func scaleStatusErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}
