package plugin

import (
	"fmt"
	"math"
	"strings"
)

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

type Query struct {
	Metric    string
	Job       string
	Group     string
	Operation string
}

func (a *APMPlugin) Query(q string) (float64, error) {
	query, err := parseQuery(q)
	if err != nil {
		return 0, fmt.Errorf("failed to parse query: %v", err)
	}

	a.logger.Debug("expanded query", "from", q, "to", fmt.Sprintf("%# v", query))

	var resp Metrics
	_, err = a.client.Raw().Query("/v1/metrics", &resp, nil)
	if err != nil {
		return 0, err
	}

	metrics := []Gauge{}
	for _, g := range resp.Gauges {
		if g.Name == query.Metric && g.Labels["job"] == query.Job && g.Labels["task_group"] == query.Group {
			metrics = append(metrics, g)
		}
	}

	if len(metrics) == 0 {
		return 0, fmt.Errorf("metric not found: %s", q)
	}

	var result float64
	switch query.Operation {
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

func parseQuery(q string) (*Query, error) {
	mainParts := strings.SplitN(q, "/", 3)
	if len(mainParts) != 3 {
		return nil, fmt.Errorf("expected <query>/<job>/group>, received %s", q)
	}

	query := &Query{
		Group: mainParts[1],
		Job:   mainParts[2],
	}

	opMetricParts := strings.Split(mainParts[0], "_")
	if len(opMetricParts) < 2 {
		return nil, fmt.Errorf(`expected <operation>_<metric>, received "%s"`, mainParts[0])
	}

	op := opMetricParts[0]
	metric := strings.Join(opMetricParts[1:], "_")

	switch metric {
	case "cpu":
		query.Metric = "nomad.client.allocs.cpu.total_percent"
	case "memory":
		query.Metric = "nomad.client.allocs.memory.usage"
	default:
		query.Metric = metric
	}

	switch op {
	case "sum":
		fallthrough
	case "avg":
		fallthrough
	case "min":
		fallthrough
	case "max":
		query.Operation = op
	default:
		return nil, fmt.Errorf(`invalid operation "%s", allowed values are sum, avg, min or max`, op)
	}

	return query, nil
}
