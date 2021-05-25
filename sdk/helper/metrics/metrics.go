package metrics

import (
	"sync/atomic"
	"time"

	m "github.com/armon/go-metrics"
)

// defaultLabels are the label set that should be applied to every data point
// emitted. Use an atomic value so that we protect against concurrent access in
// situations where the default labels are being updated after telemetry
// initialization.
var defaultLabels atomic.Value

// Label is a wrapper around m.Label so the autoscaler doesn't have to juggle
// importing both packages when emitting metrics.
type Label = m.Label

// SetDefaultLabels sets defaultLabels with the configured default set of
// labels.
func SetDefaultLabels(labels []Label) { defaultLabels.Store(labels) }

// SetGauge wraps m.SetGaugeWithLabels and sets the default labels on the
// emitted metric.
func SetGauge(key []string, val float32) {
	m.SetGaugeWithLabels(key, val, defaultLabels.Load().([]Label))
}

// MeasureSinceWithLabels wraps m.MeasureSinceWithLabels and appends the
// default labels to the passed labels on the emitted metric.
func MeasureSinceWithLabels(key []string, start time.Time, labels []Label) {
	m.MeasureSinceWithLabels(key, start, append(labels, defaultLabels.Load().([]Label)...))
}

// IncrCounter wraps m.IncrCounterWithLabels and sets the default labels on the
// emitted metric.
func IncrCounter(key []string, val float32) {
	m.IncrCounterWithLabels(key, val, defaultLabels.Load().([]Label))
}

// IncrCounterWithLabels wraps m.IncrCounterWithLabels and appends the default
// labels to the passed labels on the emitted metric.
func IncrCounterWithLabels(key []string, val float32, labels []Label) {
	m.IncrCounterWithLabels(key, val, append(labels, defaultLabels.Load().([]Label)...))
}
