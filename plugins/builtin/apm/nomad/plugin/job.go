package plugin

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
)

// taskGroupQuery is the plugins internal representation of a query and
// contains all the information needed to perform a Nomad APM query for a task
// group.
type taskGroupQuery struct {
	metric    string
	job       string
	group     string
	operation string
}

func (a *APMPlugin) queryTaskGroup(q string) (sdk.TimestampedMetrics, error) {

	// Parse the query ensuring we have all information available to make all
	// subsequent calls.
	query, err := parseTaskGroupQuery(q)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %v", err)
	}
	a.logger.Debug("expanded query", "from", q, "to", fmt.Sprintf("%# v", query))

	metrics, err := a.getTaskGroupResourceUsage(query)
	if err != nil {
		return nil, err
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("metric not found: %s", q)
	}
	a.logger.Debug("metrics found", "num_data_points", len(metrics), "query", q)

	return calculateTaskGroupResult(query.operation, metrics), nil
}

// getTaskGroupResourceUsage iterates the allocations within a job and
// identifies those which meet the criteria for being part of the calculation.
func (a *APMPlugin) getTaskGroupResourceUsage(query *taskGroupQuery) ([]float64, error) {

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

		// Since the Nomad API does not provide a metric for the percentage of CPU used
		// out of amount allocated for taskgroups, the calculation must be done here.
		// The total CPU allocated to the task group is retrieved once here since it
		// does not vary between allocations.
		allocatedCPU, err := a.getAllocatedCPUForTaskGroup(query.job, query.group)
		if err != nil {
			return nil, fmt.Errorf("failed to get total alloacted CPU for taskgroup: %v", err)
		}

		// Create the metric function now that the total allocated CPU is known
		metricFunc = func(m *[]float64, ru *api.ResourceUsage) {
			*m = append(*m, (ru.CpuStats.TotalTicks / float64(allocatedCPU)) * 100)
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

// getAllocatedCPUForTaskGroup calculates the total allocated CPU in MHz for a taskgroup
func (a *APMPlugin) getAllocatedCPUForTaskGroup(job, taskgroup string) (int, error) {
	jobInfo, _, err := a.client.Jobs().Info(job, nil)
	if err != nil {
		return -1, fmt.Errorf("failed to get info for job: %v", err)
	}

	var taskGroupConfig *api.TaskGroup
	for _, taskGroup := range jobInfo.TaskGroups {
		if taskGroup.Name != nil && *taskGroup.Name == taskgroup {
			taskGroupConfig = taskGroup
			break
		}
	}
	if taskGroupConfig == nil {
		return -1, fmt.Errorf("taskgroup was not found in job config")
	}

	taskGroupAllocatedCPU := 0
	for _, task := range taskGroupConfig.Tasks {
		if task.Resources == nil || task.Resources.CPU == nil {
			continue
		}
		taskGroupAllocatedCPU += *task.Resources.CPU
	}
	return taskGroupAllocatedCPU, nil
}

// calculateTaskGroupResult determines the query result based on the metrics
// and operation to perform.
func calculateTaskGroupResult(op string, metrics []float64) sdk.TimestampedMetrics {

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

	tm := sdk.TimestampedMetric{
		Timestamp: time.Now(),
		Value:     result,
	}
	return sdk.TimestampedMetrics{tm}
}

// parseTaskGroupQuery takes the query string and transforms it into our
// internal query representation. Parsing validates that the returned query is
// usable by all subsequent calls but cannot ensure the job or group will
// actually be found on the cluster.
func parseTaskGroupQuery(q string) (*taskGroupQuery, error) {
	mainParts := strings.SplitN(q, "/", 3)
	if len(mainParts) != 3 {
		return nil, fmt.Errorf("expected <query>/<group>/<job>, received %s", q)
	}

	query := &taskGroupQuery{
		group: mainParts[1],
		job:   mainParts[2],
	}

	opMetricParts := strings.SplitN(mainParts[0], "_", 3)
	if len(opMetricParts) != 3 {
		return nil, fmt.Errorf(`expected taskgroup_<operation>_<metric>, received "%s"`, mainParts[0])
	}

	op := opMetricParts[1]
	metric := opMetricParts[2]

	if err := validateMetric(metric); err != nil {
		return nil, err
	}
	query.metric = metric

	switch op {
	case queryOpSum, queryOpAvg, queryOpMin, queryOpMax:
		query.operation = op
	default:
		return nil, fmt.Errorf(`invalid operation %q, allowed values are %s, %s, %s or %s`,
			op, queryOpSum, queryOpAvg, queryOpMin, queryOpMax)
	}

	return query, nil
}
