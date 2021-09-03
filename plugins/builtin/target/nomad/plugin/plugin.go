package nomad

import (
	"fmt"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/sdk/helper/nomad"
	"github.com/hashicorp/nomad/api"
)

const (
	// pluginName is the unique name of the this plugin amongst target
	// plugins.
	pluginName = "nomad-target"

	// configKeys are the accepted configuration map keys which can be
	// processed when performing SetConfig().
	configKeyJobID     = "Job"
	configKeyGroup     = "Group"
	configKeyNamespace = "Namespace"

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
		PluginType: sdk.PluginTypeTarget,
	}

	PluginConfig = &plugins.InternalPluginConfig{
		Factory: func(l hclog.Logger) interface{} { return NewNomadPlugin(l) },
	}

	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeTarget,
	}

	// statusHandlerInitTimeout is the time limit a status handler must
	// initialize before considering the operation a failure.
	// Declare it as a var instead of a const so we can overwrite in tests.
	statusHandlerInitTimeout = 30 * time.Second
)

// Assert that TargetPlugin meets the target.Target interface.
var _ target.Target = (*TargetPlugin)(nil)

// TargetPlugin is the Nomad implementation of the target.Target interface.
type TargetPlugin struct {
	client *api.Client
	logger hclog.Logger

	// statusHandlers is a mapping of jobScaleStatusHandlers keyed by the
	// namespacedJobID that the handler represents. The lock should be used
	// when accessing the map.
	statusHandlers     map[namespacedJobID]*jobScaleStatusHandler
	statusHandlersLock sync.RWMutex

	// gcRunning indicates whether the GC loop is running or not.
	gcRunning     bool
	gcRunningLock sync.RWMutex
}

// namespacedJobID encapsulates the namespace and jobID, which together make a
// unique job reference within a Nomad region.
type namespacedJobID struct {
	namespace, job string
}

// NewNomadPlugin returns the Nomad implementation of the target.Target
// interface.
func NewNomadPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger:         log,
		statusHandlers: make(map[namespacedJobID]*jobScaleStatusHandler),
	}
}

// SetConfig satisfies the SetConfig function on the base.Base interface.
func (t *TargetPlugin) SetConfig(config map[string]string) error {
	t.gcRunningLock.RLock()
	defer t.gcRunningLock.RUnlock()

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

// PluginInfo satisfies the PluginInfo function on the base.Base interface.
func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

// Scale satisfies the Scale function on the target.Target interface.
func (t *TargetPlugin) Scale(action sdk.ScalingAction, config map[string]string) error {
	var countIntPtr *int
	if action.Count != sdk.StrategyActionMetaValueDryRunCount {
		countInt := int(action.Count)
		countIntPtr = &countInt
	}

	// Setup the Nomad write options.
	q := api.WriteOptions{}

	// If namespace is included within the config, add this to write opts. If
	// this is omitted, we fallback to Nomad standard practice.
	if namespace, ok := config[configKeyNamespace]; ok {
		q.Namespace = namespace
	}

	_, _, err := t.client.Jobs().Scale(config[configKeyJobID],
		config[configKeyGroup],
		countIntPtr,
		action.Reason,
		action.Error,
		action.Meta,
		&q)

	if err != nil {
		return fmt.Errorf("failed to scale group %s/%s: %v", config["job_id"], config["group"], err)
	}
	return nil
}

// Status satisfies the Status function on the target.Target interface.
func (t *TargetPlugin) Status(config map[string]string) (*sdk.TargetStatus, error) {

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

	// Attempt to find the namespace config parameter. If this is not included
	// use the Nomad default namespace "default".
	namespace, ok := config[configKeyNamespace]
	if !ok || namespace == "" {
		namespace = "default"
	}

	nsID := namespacedJobID{namespace: namespace, job: jobID}

	// Create a read/write lock on the handlers so we can safely interact.
	t.statusHandlersLock.Lock()
	defer t.statusHandlersLock.Unlock()

	// Create a handler for the job if one does not currently exist.
	if _, ok := t.statusHandlers[nsID]; !ok {
		t.statusHandlers[nsID] = newJobScaleStatusHandler(t.client, namespace, jobID, t.logger)
	}

	// If the handler is not in a running state, start it and wait for the
	// first run to finish.
	if !t.statusHandlers[nsID].isRunning {

		// doneCh is used to stop the goroutine in case it fails to initialize
		// before the timeout.
		doneCh := make(chan struct{})
		go t.statusHandlers[nsID].start(doneCh)

		// Set a timeout to make sure we don't block statusHandlers.Lock.
		select {
		case <-t.statusHandlers[nsID].initialDone:
		case <-time.After(statusHandlerInitTimeout):
			close(doneCh)
			delete(t.statusHandlers, nsID)
			return nil, fmt.Errorf("timeout while waiting for job scale status")
		}
	}

	// Return the status data from the handler to the caller.
	return t.statusHandlers[nsID].status(group)
}

// garbageCollectionLoop runs a long lived loop, triggering the garbage
// collector at a specified interval.
func (t *TargetPlugin) garbageCollectionLoop() {

	// Setup the ticker and set that the loop is now running.
	ticker := time.NewTicker(garbageCollectionSecondInterval * time.Second)

	t.gcRunningLock.Lock()
	t.gcRunning = true
	t.gcRunningLock.Unlock()

	for range ticker.C {
		t.logger.Debug("triggering run of handler garbage collection")
		t.garbageCollect()
	}
}

// garbageCollect runs a single round of status handler garbage collection.
func (t *TargetPlugin) garbageCollect() {

	// Generate the GC threshold based on the current time.
	threshold := time.Now().UTC().UnixNano() - garbageCollectionNanoSecondThreshold

	// Iterate all the handlers, ensuring we lock for safety.
	t.statusHandlersLock.Lock()
	defer t.statusHandlersLock.Unlock()

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
}
