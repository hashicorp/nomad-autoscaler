package plugin

import (
	"fmt"
	"math"
	"strings"

	"github.com/hashicorp/nomad/api"
)

// query is the plugins internal representation of a query and contains all the
// information needed to perform a Nomad APM query.
type query struct {
	metric    string
	job       string
	group     string
	operation string
}

const (
	// queryOps are the supported operators.
	queryOpSum = "sum"
	queryOpAvg = "avg"
	queryOpMax = "max"
	queryOpMin = "min"

	// queryMetrics are the supported resources for querying.
	queryMetricCPU = "cpu"
	queryMetricMem = "memory"
)

func (a *APMPlugin) Query(q string) (float64, error) {

	// Parse the query ensuring we have all information available to make all
	// subsequent calls.
	query, err := parseQuery(q)
	if err != nil {
		return 0, fmt.Errorf("failed to parse query: %v", err)
	}
	a.logger.Debug("expanded query", "from", q, "to", fmt.Sprintf("%# v", query))

	metrics, err := a.getTaskGroupResourceUsage(query)
	if err != nil {
		return 0, err
	}

	if len(metrics) == 0 {
		return 0, fmt.Errorf("metric not found: %s", q)
	}
	a.logger.Debug("metrics found", "num_data_points", len(metrics), "query", q)

	return calculateResult(query.operation, metrics), nil
}

// getTaskGroupResourceUsage iterates the allocations within a job and
// identifies those which meet the criteria for being part of the calculation.
func (a *APMPlugin) getTaskGroupResourceUsage(query *query) ([]float64, error) {

	// Grab the list of allocations assigned to the job in question.
	allocs, _, err := a.client.Jobs().Allocations(query.job, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get alloc listing for job: %v", err)
	}

	// The response is a list of data points from each allocation running in
	// the task group.
	var resp []float64

	// Define a function that manages updating our response.
	metricFunc := func(m *[]float64, ru *api.ResourceUsage) {}

	// Depending on the desired metric, the function will append different data
	// to the response. Using a function means we only have to perform the
	// switch a single time, rather than on a per allocation basis.
	switch query.metric {
	case queryMetricCPU:
		metricFunc = func(m *[]float64, ru *api.ResourceUsage) {
			*m = append(*m, ru.CpuStats.Percent)
		}
	case queryMetricMem:
		metricFunc = func(m *[]float64, ru *api.ResourceUsage) {
			*m = append(*m, float64(ru.MemoryStats.Usage))
		}
	}

	for _, alloc := range allocs {

		// If the allocation is not running, or is not part of the target task
		// group then we should skip and move onto the next allocation.
		if alloc.ClientStatus != api.AllocClientStatusRunning || alloc.TaskGroup != query.group {
			continue
		}

		// Obtains the statistics for the task group allocation. If we get a
		// single error during the iteration, we cannot reliably make a scaling
		// calculation.
		//
		// When calling Stats an entire Allocation object is needed, but only
		// the ID is used within the call. Further details:
		// https://github.com/hashicorp/nomad/issues/7955
		allocStats, err := a.client.Allocations().Stats(&api.Allocation{ID: alloc.ID}, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get alloc stats: %v", err)
		}

		// Be safe, be sensible.
		if allocStats == nil {
			continue
		}

		// Call the metric function to append the allocation resource metric to
		// the response.
		metricFunc(&resp, allocStats.ResourceUsage)
	}

	return resp, nil
}

// calculateResult determines the query result based on the metrics and
// operation to perform.
func calculateResult(op string, metrics []float64) float64 {

	var result float64

	switch op {
	case queryOpSum:
		for _, m := range metrics {
			result += m
		}
	case queryOpAvg:
		for _, m := range metrics {
			result += m
		}
		result /= float64(len(metrics))
	case queryOpMax:
		result = math.SmallestNonzeroFloat64
		for _, m := range metrics {
			if m > result {
				result = m
			}
		}
	case queryOpMin:
		result = math.MaxFloat64
		for _, m := range metrics {
			if m < result {
				result = m
			}
		}
	}
	return result
}

// parseQuery takes the query string and transforms it into our internal query
// representation. Parsing validates that the returned query is usable by all
// subsequent calls but cannot ensure the job or group will actually be found
// on the cluster.
func parseQuery(q string) (*query, error) {
	mainParts := strings.SplitN(q, "/", 3)
	if len(mainParts) != 3 {
		return nil, fmt.Errorf("expected <query>/<job>/group>, received %s", q)
	}

	query := &query{
		group: mainParts[1],
		job:   mainParts[2],
	}

	opMetricParts := strings.SplitN(mainParts[0], "_", 2)
	if len(opMetricParts) != 2 {
		return nil, fmt.Errorf(`expected <operation>_<metric>, received "%s"`, mainParts[0])
	}

	op := opMetricParts[0]
	metric := opMetricParts[1]

	switch metric {
	case queryMetricCPU, queryMetricMem:
		query.metric = metric
	default:
		return nil, fmt.Errorf(`invalid metric %q, allowed values are %s or %s`,
			metric, queryMetricCPU, queryMetricMem)
	}

	switch op {
	case queryOpSum, queryOpAvg, queryOpMin, queryOpMax:
		query.operation = op
	default:
		return nil, fmt.Errorf(`invalid operation %q, allowed values are %s, %s, %s or %s`,
			op, queryOpSum, queryOpAvg, queryOpMin, queryOpMax)
	}

	return query, nil
}
