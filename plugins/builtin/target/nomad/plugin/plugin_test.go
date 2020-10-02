package nomad

import (
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestTargetPlugin_garbageCollect(t *testing.T) {

	curTime := time.Now().UTC().UnixNano()
	testName := "generic GC test"

	// Build the plugin with some populated handlers and data to test.
	targetPlugin := TargetPlugin{
		logger: hclog.NewNullLogger(),
		statusHandlers: map[namespacedJobID]*jobScaleStatusHandler{
			namespacedJobID{"default", "running"}:               {isRunning: true, lastUpdated: curTime},
			namespacedJobID{"default", "recently-stopped"}:      {isRunning: false, lastUpdated: curTime - 1800000000000},
			namespacedJobID{"default", "stopped-long-time-ago"}: {isRunning: false, lastUpdated: curTime - 18000000000000},
			namespacedJobID{"special", "running"}:               {isRunning: true, lastUpdated: curTime},
			namespacedJobID{"special", "recently-stopped"}:      {isRunning: false, lastUpdated: curTime - 1800000000000},
			namespacedJobID{"special", "stopped-long-time-ago"}: {isRunning: false, lastUpdated: curTime - 18000000000000},
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
