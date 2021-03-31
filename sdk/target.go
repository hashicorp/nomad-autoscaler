package sdk

// TargetStatus is the response object when performing the Status call of the
// target plugin interface. The response details key information about the
// current state of the target.
type TargetStatus struct {

	// Ready indicates whether the target is currently in a state where scaling
	// is permitted.
	Ready bool

	// Count is the current value of the target and thus performs the current
	// state basis when performing strategy calculations to identify the
	// desired state.
	Count int64

	// Meta is a mapping that provides additional information about the target
	// that can be used during the policy evaluation to ensure the correct
	// calculations and logic are applied to the target.
	Meta map[string]string
}

const (
	// TargetStatusMetaKeyLastEvent is an optional meta key that can be added
	// to the status return. The value represents the last scaling event of the
	// target as seen by the remote providers view point. This helps enforce
	// cooldown where out-of-band scaling activities have been triggered.
	TargetStatusMetaKeyLastEvent = "nomad_autoscaler.last_event"

	// TargetConfigKeyJob is the config key used within horizontal app scaling
	// to identify the Nomad job targeted for autoscaling.
	TargetConfigKeyJob = "Job"

	// TargetConfigKeyTaskGroup is the config key used within horizontal app
	// scaling to identify the Nomad job group targeted for autoscaling.
	TargetConfigKeyTaskGroup = "Group"

	// TargetConfigKeyClass is the config key used with horizontal cluster
	// scaling to identify Nomad clients as part of a pool of resources. This
	// pool of resources forms the scalable target.
	TargetConfigKeyClass = "node_class"

	// TargetConfigKeyDrainDeadline is the config key which defines the
	// override value to use when draining a Nomad client during the scale in
	// action of horizontal cluster scaling.
	TargetConfigKeyDrainDeadline = "node_drain_deadline"

	// TargetConfigKeyIgnoreSystemJobs is the config key which defines whether or not
	// nomad system jobs are drained during the drain operation
	TargetConfigKeyIgnoreSystemJobs = "node_drain_ignore_system_jobs"

	// TargetConfigKeyNodePurge is the config key which defines whether or not
	// Nomad clients are purged from Nomad once they have been terminated
	// within their provider.
	TargetConfigKeyNodePurge = "node_purge"

	// TargetConfigNodeSelectorStrategy is the optional node target config
	// option which dictates how the Nomad Autoscaler selects nodes when
	// scaling in.
	TargetConfigNodeSelectorStrategy = "node_selector_strategy"
)

const (
	// TargetNodeSelectorStrategyLeastBusy is the cluster scale-in node
	// selection strategy that identifies nodes based on their CPU and Memory
	// resource allocation. It picks those with lowest values and is the
	// default strategy.
	TargetNodeSelectorStrategyLeastBusy = "least_busy"

	// TargetNodeSelectorStrategyNewestCreateIndex is the cluster scale-in node
	// selection strategy that uses the sorting perform by the SDK. This
	// sorting is based on the nodes CreateIndex, where nodes with higher
	// indexes are selected. This is the fastest of the strategies and should
	// be used if you have performance concerns or do not care about node
	// selection.
	TargetNodeSelectorStrategyNewestCreateIndex = "newest_create_index"

	// TargetNodeSelectorStrategyEmpty is the cluster scale-in node selection
	// strategy that only picks nodes without non-terminal allocations.
	TargetNodeSelectorStrategyEmpty = "empty"
)
