package strategy

import "fmt"

const (
	// Standarized Meta keys used by the Autoscaler.
	metaKeyDryRun        = "nomad_autoscaler.dry_run"
	metaKeyDryRunCount   = "nomad_autoscaler.dry_run.count"
	metaKeyCountCapped   = "nomad_autoscaler.count.capped"
	metaKeyCountOriginal = "nomad_autoscaler.count.original"
	metaKeyReasonHistory = "nomad_autoscaler.reason_history"

	// MetaValueDryRunCount is a special count value used when performing
	// dry-run scaling activities. The Autoscaler will never set a count to a
	// negative value during normal operation, so the agent is safe to assume a
	// count set to this value implies dry-run.
	MetaValueDryRunCount = -1
)

// Action represents a Strategy's intention to modify.
type Action struct {

	// Count represents the desired count of the target resource. It should
	// always be zero or above, expect in the event of dry-run where it can use
	// the MetaValueDryRunCount value.
	Count int64

	Reason string
	Error  bool
	Meta   map[string]interface{}
}

// Canonicalize ensures Action has proper default values.
func (a *Action) Canonicalize() {
	if a.Meta == nil {
		a.Meta = make(map[string]interface{})
	}
}

// SetDryRun marks the Action to be executed in dry-run mode. Dry-run mode is
// indicated using Meta tags. A dry-run action doesn't modify the Target's
// count value.
func (a *Action) SetDryRun() {
	a.Meta[metaKeyDryRun] = true
	a.Meta[metaKeyDryRunCount] = a.Count
	a.Count = MetaValueDryRunCount
}

// CapCount caps the value of Count so it remains within the specified limits.
func (a *Action) CapCount(min, max int64) {

	oldCount, newCount := a.Count, a.Count
	if newCount < min {
		newCount = min
	} else if newCount > max {
		newCount = max
	}

	if newCount != oldCount {
		a.Meta[metaKeyCountCapped] = true
		r := fmt.Sprintf("capped count from %d to %d to stay within limits", oldCount, newCount)
		a.handleCountChange(oldCount, newCount, r)
	}
}

// LimitChange is used to modify the desired action based on the operator
// specified maximum allowed change.
func (a *Action) LimitChange(currentCount, maxChange int64) {

	oldCount, newCount := a.Count, a.Count

	// Determine whether this action is scaling in or out so we can correctly
	// perform our calculations.
	if a.Count < currentCount {
		if change := currentCount - a.Count; change > maxChange {
			newCount = currentCount - maxChange
		}
	} else if a.Count > currentCount {
		if change := a.Count - currentCount; change > maxChange {
			newCount = currentCount + maxChange
		}
	}

	// If we limited the scope of the change, update the action count and
	// insert a reason for the modification.
	if newCount != oldCount {
		r := fmt.Sprintf("modified desired count from %d to %d to limit change to %d",
			oldCount, newCount, maxChange)
		a.handleCountChange(oldCount, newCount, r)
	}
}

// handleCountChange is a helper function which encompasses updates to an
// action when the count needs to be changed by limits or thresholds. This
// importantly allows for any functions which perform limit checks or capping
// to be run in any order.
func (a *Action) handleCountChange(oldCount, newCount int64, reason string) {

	// If we don't have an existing original count, add the meta parameter. We
	// don't want to overwrite the value as writing a single time ensures we
	// have the real original desired count.
	if _, ok := a.Meta[metaKeyCountOriginal]; !ok {
		a.Meta[metaKeyCountOriginal] = oldCount
	}

	// Most importantly, set our new count.
	a.Count = newCount

	// Store the reason for the change in our action object.
	a.pushReason(reason)
}

// pushReason updates the Reason value and stores previous Reason into Meta.
func (a *Action) pushReason(r string) {
	history := []string{}

	// Check if we already have a reason stack in Meta
	if historyInterface, ok := a.Meta[metaKeyReasonHistory]; ok {
		if historySlice, ok := historyInterface.([]string); ok {
			history = historySlice
		}
	}

	// Append current reason to history and update action.
	if a.Reason != "" {
		history = append(history, a.Reason)
	}
	a.Meta[metaKeyReasonHistory] = history
	a.Reason = r
}
