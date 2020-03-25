package nomad

import (
	"fmt"
	"math"
	"strings"

	nomadHelper "github.com/hashicorp/nomad-autoscaler/helper/nomad"
	"github.com/hashicorp/nomad/api"
)

type MetricsAPM struct {
	client *api.Client
}

type Metrics struct {
	Counters []Counter
	Gauges   []Gauge
	Samples  []Sample
}

type Counter struct {
	Count  float64
	Labels map[string]string
	Max    float64
	Mean   float64
	Min    float64
	Name   string
	Rate   float64
	Stddev float64
	Sum    float64
}

type Gauge struct {
	Labels map[string]string
	Name   string
	Value  float64
}

type Sample struct {
	Counter
}

func (m *MetricsAPM) SetConfig(config map[string]string) error {

	cfg := nomadHelper.ConfigFromMap(config)

	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to instantiate Nomad client: %v", err)
	}
	m.client = client

	return nil
}

func (m *MetricsAPM) Query(q string) (float64, error) {
	parts := strings.Split(q, "/")
	if len(parts) != 4 {
		return 0, fmt.Errorf("invalid query %s, expected 4 parts, got %d", q, len(parts))
	}

	metric := parts[0]
	job := parts[1]
	group := parts[2]
	op := parts[3]

	var resp Metrics
	_, err := m.client.Raw().Query("/v1/metrics", &resp, nil)
	if err != nil {
		return 0, err
	}

	metrics := []Gauge{}
	for _, g := range resp.Gauges {
		if g.Name == metric && g.Labels["job"] == job && g.Labels["task_group"] == group {
			metrics = append(metrics, g)
		}
	}

	if len(metrics) == 0 {
		return 0, fmt.Errorf("metric not found: %s", q)
	}

	var result float64
	switch op {
	case "sum":
		for _, m := range metrics {
			result += m.Value
		}
	case "avg":
		for _, m := range metrics {
			result += m.Value
		}
		result /= float64(len(metrics))
	case "max":
		result = math.SmallestNonzeroFloat64
		for _, m := range metrics {
			if m.Value > result {
				result = m.Value
			}
		}
	case "min":
		result = math.MaxFloat64
		for _, m := range metrics {
			if m.Value < result {
				result = m.Value
			}
		}
	}

	return result, nil
}
