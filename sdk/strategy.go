package sdk

import "fmt"

const (
	// strategyActionMetaKey are standardised keys used by the autoscaler to
	// populate the ScalingAction Meta mapping with useful information for
	// operators.
	strategyActionMetaKeyDryRun        = "nomad_autoscaler.dry_run"
	strategyActionMetaKeyDryRunCount   = "nomad_autoscaler.dry_run.count"
	strategyActionMetaKeyCountCapped   = "nomad_autoscaler.count.capped"
	strategyActionMetaKeyCountOriginal = "nomad_autoscaler.count.original"
	strategyActionMetaKeyReasonHistory = "nomad_autoscaler.reason_history"

	// StrategyActionMetaValueDryRunCount is a special count value used when
	// performing dry-run scaling activities. The Autoscaler will never set a
	// count to a negative value during normal operation, so the agent is safe
	// to assume a count set to this value implies dry-run.
	StrategyActionMetaValueDryRunCount = -1
)

// ScalingAction represents a strategy plugins intention to change the current
// target state. It includes all the required information to enact the change,
// along with useful meta information for operators and admins.
type ScalingAction struct {

	// Count represents the desired count of the target resource. It should
	// always be zero or above, expect in the event of dry-run where it can use
	// the StrategyActionMetaValueDryRunCount value.
	Count int64

	// Reason is the top level string that provides a user friendly description
	// of why the strategy decided the action was required.
	Reason string

	// Error indicates whether the Reason string is an error condition. This
	// allows the Reason to be flexible in its use.
	Error bool

	// Direction is the scaling direction the strategy has decided should
	// happen. This is particularly helpful for non-Nomad target
	// implementations whose APIs dead with increment changes rather than
	// absolute counts.
	Direction ScaleDirection

	// Meta
	Meta map[string]interface{}
}

// ScaleDirection is an identifier used by strategy plugins to identify how the
// target should scale the named resource.
type ScaleDirection int8

// The following constants are used to standardize the possible scaling
// directions for an Action. They are ordered from riskier to safest, with
// ScaleDirectionNone as the default and zero value.
const (
	// ScaleDirectionDown indicates the target should lower the number of running
	// instances of the resource.
	ScaleDirectionDown = iota - 1

	// ScaleDirectionNone indicates no scaling is required.
	ScaleDirectionNone

	// ScaleDirectionUp indicates the target should increase the number of
	// running instances of the resource.
	ScaleDirectionUp
)

// String satisfies the Stringer interface and returns as string representation
// of the scaling direction.
func (d ScaleDirection) String() string {
	switch d {
	case ScaleDirectionDown:
		return "down"
	case ScaleDirectionUp:
		return "up"
	default:
		return "none"
	}
}

// Canonicalize ensures Action has proper default values.
func (a *ScalingAction) Canonicalize() {
	if a.Meta == nil {
		a.Meta = make(map[string]interface{})
	}
}

// SetDryRun marks the Action to be executed in dry-run mode. Dry-run mode is
// indicated using Meta tags. A dry-run action doesn't modify the Target's
// count value.
func (a *ScalingAction) SetDryRun() {
	a.Meta[strategyActionMetaKeyDryRun] = true
	a.Meta[strategyActionMetaKeyDryRunCount] = a.Count
	a.Count = StrategyActionMetaValueDryRunCount
}

// CapCount caps the value of Count so it remains within the specified limits.
// If Count is StrategyActionMetaValueDryRunCount this method has no effect.
func (a *ScalingAction) CapCount(min, max int64) {
	if a.Count == StrategyActionMetaValueDryRunCount {
		return
	}

	oldCount, newCount := a.Count, a.Count
	if newCount < min {
		newCount = min
	} else if newCount > max {
		newCount = max
	}

	if newCount != oldCount {
		a.Meta[strategyActionMetaKeyCountCapped] = true
		a.Meta[strategyActionMetaKeyCountOriginal] = oldCount
		a.pushReason(fmt.Sprintf("capped count from %d to %d to stay within limits", oldCount, newCount))
		a.Count = newCount
	}
}

// PushReason updates the Reason value and stores previous Reason into Meta.
func (a *ScalingAction) pushReason(r string) {
	history := []string{}

	// Check if we already have a reason stack in Meta
	if historyInterface, ok := a.Meta[strategyActionMetaKeyReasonHistory]; ok {
		if historySlice, ok := historyInterface.([]string); ok {
			history = historySlice
		}
	}

	// Append current reason to history and update action.
	if a.Reason != "" {
		history = append(history, a.Reason)
	}
	a.Meta[strategyActionMetaKeyReasonHistory] = history
	a.Reason = r
}

// PreemptScalingAction determines which ScalingAction should take precedence.
//
// The result is based on the scaling direction and count. The order of
// precedence for the scaling directions is defined by the order in which they
// are declared in the above enum.
//
// If the scaling direction is the same, the priority is given to the safest
// option, where safest is defined as lowest impact in the underlying
// infrastructure:
//
// * ScaleDirectionUp: Action with highest count
// * ScaleDirectionDown: Action with highest count
func PreemptScalingAction(a *ScalingAction, b *ScalingAction) *ScalingAction {
	if a == nil {
		return b
	}

	if b == nil {
		return a
	}

	if b.Direction > a.Direction {
		return b
	}

	if a.Direction == b.Direction {
		switch a.Direction {
		case ScaleDirectionUp, ScaleDirectionDown:
			if b.Count > a.Count {
				return b
			}
		}
	}

	return a
}
