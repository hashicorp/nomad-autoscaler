package sdk

import "time"

// TimestampedMetric contains a single metric Value along with its associated
// Timestamp.
type TimestampedMetric struct {
	Timestamp time.Time
	Value     float64
}

// TimestampedMetrics is an array of timestamped metric values. This type is
// used so we can sort metrics based on the timestamp.
type TimestampedMetrics []TimestampedMetric

// Len satisfies the Len function of the sort.Interface interface.
func (t TimestampedMetrics) Len() int { return len(t) }

// Less satisfies the Less function of the sort.Interface interface.
func (t TimestampedMetrics) Less(i, j int) bool { return t[i].Timestamp.Before(t[j].Timestamp) }

// Swap satisfies the Swap function of the sort.Interface interface.
func (t TimestampedMetrics) Swap(i, j int) { t[i], t[j] = t[j], t[i] }

// TimeRange defines a range of time.
type TimeRange struct {
	From time.Time
	To   time.Time
}
