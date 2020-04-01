package strategy

import "fmt"

const (
	metaKeyDryRun        = "nomad_autoscaler.dry_run"
	metaKeyDryRunCount   = "nomad_autoscaler.dry_run.count"
	metaKeyCountCapped   = "nomad_autoscaler.count.capped"
	metaKeyCountOriginal = "nomad_autoscaler.count.original"
	metaKeyCountNil      = "nomad_autoscaler.count.nil"
	metaKeyReasonHistory = "nomad_autoscaler.reason_history"
)

type Action struct {
	Count  *int64
	Reason string
	Meta   map[string]interface{}
}

func (a *Action) Canonicalize() {
	if a.Meta == nil {
		a.Meta = make(map[string]interface{})
	}

	if a.Count == nil {
		a.Meta[metaKeyCountNil] = true
	}
}

func (a *Action) SetDryRun(dryRun bool) {
	a.Meta[metaKeyDryRun] = true
	if a.Count != nil {
		a.Meta[metaKeyDryRunCount] = *a.Count
	}
	a.Count = nil
}

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
		a.PushReason(fmt.Sprintf("capped count from %d to %d to stay withing limits", oldCount, newCount))
		a.Count = &newCount
	}
}

func (a *Action) PushReason(r string) {
	history := []string{}

	// Check if we already have a reason stack in Meta
	if historyInterface, ok := a.Meta[metaKeyReasonHistory]; ok {
		if historySlice, ok := historyInterface.([]string); ok {
			history = historySlice
		}
	}

	// Append current reason to history and update action
	a.Meta[metaKeyReasonHistory] = append(history, a.Reason)
	a.Reason = r
}
