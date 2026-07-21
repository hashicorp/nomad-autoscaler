// Copyright IBM Corp. 2020, 2026

package plugin

// instanaMetricsRequest is the JSON body POSTed to the Instana infrastructure
// metrics endpoint: POST /api/infrastructure-monitoring/metrics
type instanaMetricsRequest struct {
	TimeFrame   instanaTimeFrame `json:"timeFrame"`
	Plugin      string           `json:"plugin"`
	Query       string           `json:"query,omitempty"`
	SnapshotIDs []string         `json:"snapshotIds,omitempty"`
	Rollup      int32            `json:"rollup,omitempty"`
	Metrics     []string         `json:"metrics"`
}

// instanaMetricsResponse is the top-level JSON response from Instana.
type instanaMetricsResponse struct {
	Items []instanaMetricItem `json:"items"`
}

// instanaTimeFrame defines the query window sent to Instana.
// Both fields are Unix millisecond epochs; WindowSize is To minus From.
type instanaTimeFrame struct {
	WindowSize int64 `json:"windowSize"`
	To         int64 `json:"to"`
}

// instanaMetricItem represents one entity snapshot returned in the response.
// Metrics maps a metric ID to a slice of [timestamp_ms, value] pairs.
type instanaMetricItem struct {
	SnapshotID string                  `json:"snapshotId"`
	Label      string                  `json:"label"`
	Metrics    map[string][][2]float64 `json:"metrics"`
}
