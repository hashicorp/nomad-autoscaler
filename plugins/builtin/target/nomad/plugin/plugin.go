package nomad

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/plugins/strategy"
	"github.com/hashicorp/nomad-autoscaler/plugins/target"
	"github.com/hashicorp/nomad/api"
)

const (
	// pluginName is the name of the plugin
	pluginName = "nomad-target"

	configKeyJobID = "job_id"
	configKeyGroup = "group"
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

type TargetPlugin struct {
	client *api.Client
	logger hclog.Logger

	statusHandlers     map[string]*jobStateHandler
	statusHandlersLock sync.RWMutex

	gcRunning bool
}

func NewNomadPlugin(log hclog.Logger) *TargetPlugin {
	return &TargetPlugin{
		logger:         log,
		statusHandlers: make(map[string]*jobStateHandler),
	}
}

func (t *TargetPlugin) SetConfig(config map[string]string) error {

	if !t.gcRunning {
		go t.garbageCollectionLoop()
	}

	cfg := nomadHelper.ConfigFromMap(config)

	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	t.client = client

	return nil
}

func (t *TargetPlugin) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

func (t *TargetPlugin) Count(config map[string]string) (int64, error) {
	// TODO: validate if group is valid
	allocs, _, err := t.client.Jobs().Allocations(config["job_id"], false, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve Nomad job: %v", err)
	}

	var count int64
	for _, alloc := range allocs {
		if alloc.TaskGroup == config["group"] && alloc.ClientStatus == "running" {
			count++
		}
	}

	return count, nil
}

func (t *TargetPlugin) Scale(action strategy.Action, config map[string]string) error {
	var countIntPtr *int
	if action.Count != nil {
		countInt := int(*action.Count)
		countIntPtr = &countInt
	}

	_, _, err := t.client.Jobs().Scale(config["job_id"],
		config["group"],
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

func (t *TargetPlugin) Status(config map[string]string) (*target.Status, error) {

	// Get the JobID from the config map. This is a required param and results
	// in an error if not found.
	jobID, ok := config[configKeyJobID]
	if !ok {
		return nil, fmt.Errorf("required config key %q not found", configKeyJobID)
	}

	// Get the GroupName from the config map. This is a required param and
	// results in an error if not found.
	group, ok := config[configKeyGroup]
	if !ok {
		return nil, fmt.Errorf("required config key %q not found", configKeyGroup)
	}

	// Create a read/write lock on the handlers so we can safely interact.
	t.statusHandlersLock.Lock()
	defer t.statusHandlersLock.Unlock()

	// Create a handler for the job if one does not currently exist.
	if _, ok := t.statusHandlers[jobID]; !ok {
		t.statusHandlers[jobID] = newJobStateHandler(t.client, jobID, t.logger)
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

func (t *TargetPlugin) garbageCollectionLoop() {

	ticker := time.NewTicker(60 * time.Second)
	t.gcRunning = true

	for {
		select {
		case <-ticker.C:
			t.garbageCollect()
		}
	}
}

func (t *TargetPlugin) garbageCollect() {

	// Generate the GC threshold based on the current time.
	threshold := time.Now().UTC().UnixNano() - 14400000000000

	// Iterate all the handlers, ensuring we lock for safety.
	t.statusHandlersLock.Lock()

	for jobID, handle := range t.statusHandlers {

		// If the handler is running, there is no need to GC.
		if handle.isRunning {
			continue
		}

		// If the last updated time is more recent than our threshold,
		// do not GC. The handler isn't quite ready to meet its death,
		// maybe soon though.
		if handle.lastUpdated > threshold {
			continue
		}

		// If we have reached this point, the handler should be
		// removed. Goodbye old friend.
		delete(t.statusHandlers, jobID)
		t.logger.Debug("removed inactive job status handler", "job_id", jobID)
	}

	t.statusHandlersLock.Unlock()
}
