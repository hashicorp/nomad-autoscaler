package strategy

import "fmt"

const (
	// Standarized Meta keys used by the Autoscaler.
	metaKeyDryRun        = "nomad_autoscaler.dry_run"
	metaKeyDryRunCount   = "nomad_autoscaler.dry_run.count"
	metaKeyCountCapped   = "nomad_autoscaler.count.capped"
	metaKeyCountOriginal = "nomad_autoscaler.count.original"
	metaKeyReasonHistory = "nomad_autoscaler.reason_history"
)

// Action represents a Strategy's intention to modify.
type Action struct {
	Count  *int64
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
	if a.Count != nil {
		a.Meta[metaKeyDryRunCount] = *a.Count
	}
	a.Count = nil
}

// CapCount caps the value of Count so it remains within the specified limits.
// If Count is nil this method has no effect.
func (a *Action) CapCount(min, max int64) {
	if a.Count == nil {
		return
	}

	oldCount, newCount := *a.Count, *a.Count
	if newCount < min {
		newCount = min
	} else if newCount > max {
		newCount = max
	}

	if newCount != oldCount {
		a.Meta[metaKeyCountCapped] = true
		a.Meta[metaKeyCountOriginal] = oldCount
		a.pushReason(fmt.Sprintf("capped count from %d to %d to stay within limits", oldCount, newCount))
		a.Count = &newCount
	}
}

// PushReason updates the Reason value and stores previous Reason into Meta.
func (a *Action) pushReason(r string) {
	history := []string{}

	// Check if we already have a reason stack in Meta
	if historyInterface, ok := a.Meta[metaKeyReasonHistory]; ok {
		if historySlice, ok := historyInterface.([]string); ok {
			history = historySlice
		}
	}

	// Append current reason to history and update action.
	a.Reason = r
	a.Meta[metaKeyReasonHistory] = append(history, a.Reason)
}
