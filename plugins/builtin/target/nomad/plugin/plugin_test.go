package nomad

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/stretchr/testify/assert"
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

func TestTargetPlugin_statusTimeout(t *testing.T) {
	nomadMock := httptest.NewServer(http.HandlerFunc(scaleStatusHandler))
	defer nomadMock.Close()

	statusHandlerInitTimeout = 3 * time.Second

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

func scaleStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}
