package agent

import (
	"net/http"

	metrics "github.com/armon/go-metrics"
)

type MockAgentHTTP struct{}

func (m *MockAgentHTTP) DisplayMetrics(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return metrics.MetricsSummary{
		Timestamp: "2020-11-17 00:17:50 +0000 UTC",
		Counters:  []metrics.SampledValue{},
		Gauges:    []metrics.GaugeValue{},
		Points:    []metrics.PointValue{},
		Samples:   []metrics.SampledValue{},
	}, nil
}
func (m *MockAgentHTTP) ReloadAgent(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, nil
}
