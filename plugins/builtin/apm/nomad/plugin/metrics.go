package plugin

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad-autoscaler/sdk"
)

const (
	// queryTypes are the types of query the Nomad APM plugin can handle. Each
	// one has its own path to discovering the correct data and so its
	// important this is included and validated on every query request.
	QueryTypeTaskGroup = "taskgroup"
	QueryTypeNode      = "node"

	// queryOps below are the supported operators for task group queries.
	queryOpSum = "sum"
	queryOpAvg = "avg"
	queryOpMax = "max"
	queryOpMin = "min"

	// queryOps below are the supported operators for node pool queries.
	queryOpPercentageAllocated = "percentage-allocated"

	// queryMetrics are the supported resources for querying.
	queryMetricCPU = "cpu"
	queryMetricMem = "memory"
)

// Query satisfies the Query function on the apm.APM interface.
func (a *APMPlugin) Query(q string, _ sdk.TimeRange) (sdk.TimestampedMetrics, error) {

	// Split the input query so we can understand which query type we are
	// dealing with.
	querySplit := strings.Split(q, "_")

	switch querySplit[0] {
	case QueryTypeTaskGroup:
		return a.queryTaskGroup(q)
	case QueryTypeNode:
		return a.queryNodePool(q)
	default:
		return nil, fmt.Errorf("unsupported query type %q", querySplit[0])
	}
}

// validateMetric helps ensure the desired metric within the query is able to
// be handled by the plugin.
func validateMetric(metric string) error {

	var err error

	switch metric {
	case queryMetricCPU, queryMetricMem:
	default:
		err = fmt.Errorf(`invalid metric %q, allowed values are %s or %s`,
			metric, queryMetricCPU, queryMetricMem)
	}
	return err
}
