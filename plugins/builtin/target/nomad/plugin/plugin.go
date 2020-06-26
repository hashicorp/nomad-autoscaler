package nomad

import (
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad/api"
)

const (
	// pluginName is the unique name of the this plugin amongst target
	// plugins.
	pluginName = "nomad-target"

	// configKeys are the accepted configuration map keys which can be
	// processed when performing SetConfig().
	configKeyJobID = "Job"
	configKeyGroup = "Group"

	// garbageCollectionNanoSecondThreshold is the nanosecond threshold used
	// when performing garbage collection of job status handlers.
	garbageCollectionNanoSecondThreshold = 14400000000000

	// garbageCollectionSecondInterval is the interval in seconds at which the
	// garbage collector will run.
	garbageCollectionSecondInterval = 60
)

var (
	PluginID = plugins.PluginID{
		Name:       pluginName,
		PluginType: plugins.PluginTypeTarget,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewNomadPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: plugins.PluginTypeTarget,
	}
)

// Assert that TargetPlugin meets the target.Target interface.
var _ target.Target = (*TargetPlugin)(nil)

// TargetPlugin is the Nomad implementation of the target.Target interface.
type TargetPlugin struct {
	client *api.Client
	logger hclog.Logger

	// statusHandlers is a mapping of jobScaleStatusHandlers keyed by the jobID
	// that the handler represents. The lock should be used when accessing the
	// map.
	statusHandlers     map[string]*jobScaleStatusHandler
	statusHandlersLock sync.RWMutex

	// gcRunning indicates whether the GC loop is running or not.
	gcRunning bool
}

// NewNomadPlugin returns the Nomad implementation of the target.Target
// interface.
func NewNomadPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger:         log,
		statusHandlers: make(map[string]*jobScaleStatusHandler),
	}
}

// SetConfig satisfies the SetConfig function on the base.Plugin interface.
func (t *TargetPlugin) SetConfig(config map[string]string) error {

	if !t.gcRunning {
		go t.garbageCollectionLoop()
	}

	cfg := nomadHelper.ConfigFromNamespacedMap(config)

	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	t.client = client

	return nil
}

// PluginInfo satisfies the PluginInfo function on the base.Plugin interface.
func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Scale satisfies the Scale function on the target.Target interface.
func (t *TargetPlugin) Scale(action strategy.Action, config map[string]string) error {
	var countIntPtr *int
	if action.Count != strategy.MetaValueDryRunCount {
		countInt := int(action.Count)
		countIntPtr = &countInt
	}

	_, _, err := t.client.Jobs().Scale(config[configKeyJobID],
		config[configKeyGroup],
		countIntPtr,
		action.Reason,
		action.Error,
		action.Meta,
		nil)

	if err != nil {
		return fmt.Errorf("failed to scale group %s/%s: %v", config["job_id"], config["group"], err)
	}
	return nil
}

// Status satisfies the Status function on the target.Target interface.
func (t *TargetPlugin) Status(config map[string]string) (*target.Status, error) {

	// Get the JobID from the config map. This is a required param and results
	// in an error if not found or is an empty string.
	jobID, ok := config[configKeyJobID]
	if !ok || jobID == "" {
		return nil, fmt.Errorf("required config key %q not found", configKeyJobID)
	}

	// Get the GroupName from the config map. This is a required param and
	// results in an error if not found or is an empty string.
	group, ok := config[configKeyGroup]
	if !ok || group == "" {
		return nil, fmt.Errorf("required config key %q not found", configKeyGroup)
	}

	// Create a read/write lock on the handlers so we can safely interact.
	t.statusHandlersLock.Lock()
	defer t.statusHandlersLock.Unlock()

	// Create a handler for the job if one does not currently exist.
	if _, ok := t.statusHandlers[jobID]; !ok {
		t.statusHandlers[jobID] = newJobScaleStatusHandler(t.client, jobID, t.logger)
	}

	// If the handler is not in a running state, start it and wait for the
	// first run to finish.
	if !t.statusHandlers[jobID].isRunning {
		go t.statusHandlers[jobID].start()
		<-t.statusHandlers[jobID].initialDone
	}

	// Return the status data from the handler to the caller.
	return t.statusHandlers[jobID].status(group)
}

// garbageCollectionLoop runs a long lived loop, triggering the garbage
// collector at a specified interval.
func (t *TargetPlugin) garbageCollectionLoop() {

	// Setup the ticker and set that the loop is now running.
	ticker := time.NewTicker(garbageCollectionSecondInterval * time.Second)
	t.gcRunning = true

	for {
		select {
		case <-ticker.C:
			t.logger.Debug("triggering run of handler garbage collection")
			t.garbageCollect()
		}
	}
}

// garbageCollect runs a single round of status handler garbage collection.
func (t *TargetPlugin) garbageCollect() {

	// Generate the GC threshold based on the current time.
	threshold := time.Now().UTC().UnixNano() - garbageCollectionNanoSecondThreshold

	// Iterate all the handlers, ensuring we lock for safety.
	t.statusHandlersLock.Lock()

	for jobID, handle := range t.statusHandlers {

		// If the handler is running, there is no need to GC.
		if handle.isRunning {
			continue
		}

		// If the last updated time is before our threshold, the handler should
		// be removed. Goodbye old friend.
		if handle.lastUpdated < threshold {
			delete(t.statusHandlers, jobID)
			t.logger.Debug("removed inactive job status handler", "job_id", jobID)
		}
	}

	t.statusHandlersLock.Unlock()
}
