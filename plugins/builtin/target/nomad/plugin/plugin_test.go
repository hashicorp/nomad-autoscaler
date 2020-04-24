package nomad

import (
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestTargetPlugin_garbageCollect(t *testing.T) {

	curTime := time.Now().UTC().UnixNano()

	// Build the plugin with some populated handlers and data to test.
	targetPlugin := TargetPlugin{
		logger: hclog.NewNullLogger(),
		statusHandlers: map[string]*jobScaleStatusHandler{
			"running":               {isRunning: true, lastUpdated: curTime},
			"recently-stopped":      {isRunning: false, lastUpdated: curTime - 1800000000000},
			"stopped-long-time-ago": {isRunning: false, lastUpdated: curTime - 18000000000000},
		},
	}

	// Trigger the GC.
	targetPlugin.garbageCollect()

	// Perform our assertions to confirm the statusHandlers mapping has the
	// entries expected after running the GC.
	assert.Nil(t, targetPlugin.statusHandlers["stopped-long-time-ago"])
	assert.NotNil(t, targetPlugin.statusHandlers["running"])
	assert.NotNil(t, targetPlugin.statusHandlers["recently-stopped"])
	assert.Len(t, targetPlugin.statusHandlers, 2)
}
