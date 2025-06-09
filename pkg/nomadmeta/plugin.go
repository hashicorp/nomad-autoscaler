package nomadmeta

import (
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/pkg/errors"
)

type NomadMeta struct {
	logger  hclog.Logger
	counter NodeCounter
	diff    *DiffReporter
}

const (
	nodePoolDefault  = "default"
	nodePoolAllNodes = "all"
)

func NewNomadMeta(logger hclog.Logger, counter NodeCounter) *NomadMeta {
	return &NomadMeta{
		logger:  logger,
		counter: counter,
		diff:    NewDiffReporter(),
	}
}

type Query struct {
	Job       string
	Group     string
	NodePool  string
	MetaQuery string
}

func (n *NomadMeta) Query(q Query, r sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	n.logger.Debug("query request", "query", q.MetaQuery, "job", q.Job, "range", r)

	nodes, err := n.counter.GetNodeNames(q.MetaQuery, q.NodePool)
	if err != nil {
		return nil, err
	}

	ineligible, err := n.counter.RunningOnIneligibleNodes(q.Job)
	if err != nil {
		return nil, err
	}

	total := float64(len(nodes)) + float64(len(ineligible))

	n.logger.Info("query response",
		"query", q.MetaQuery,
		"pool", q.NodePool, "job",
		q.Job, "total", len(nodes),
		"ineligible", len(ineligible),
		"ineligible_nodes", ineligible,
	)

	added, removed, before := n.diff.Diff(q.Job, nodes)
	if len(added) > 0 || len(removed) > 0 {
		n.logger.Info("nodes changed", "job", q.Job, "added", added, "removed", removed, "before", before)
	}

	return sdk.TimestampedMetrics{
		{
			Timestamp: time.Now(),
			Value:     total,
		},
	}, nil

}

func (n *NomadMeta) QueryMultiple(q Query, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	m, err := n.Query(q, r)
	if err != nil {
		return nil, err
	}

	return []sdk.TimestampedMetrics{m}, nil
}

var (
	legacyRawQueryRegex = regexp.MustCompile(`\s*job="([^"]+)"\s*group="([^"]+)"\s*(.*)`)
	rawQueryRegex       = regexp.MustCompile(`\s*job="([^"]+)"\s*group="([^"]+)"\s*node_pool="([^"]+)"\s*(.*)`)
	ErrInvalidQuery     = errors.New("invalid query provided")
)

const (
	// only count nodes that are eligible and capable of running docker containers
	querySuffix = `SchedulingEligibility == "eligible" and Attributes contains "driver.docker"`
)

func ToQuery(q string) (Query, error) {
	matches := rawQueryRegex.FindStringSubmatch(q)

	if len(matches) != 5 {
		return toLegacyQuery(q)
	}

	return Query{
		Job:       matches[1],
		Group:     matches[2],
		NodePool:  matches[3],
		MetaQuery: fmt.Sprintf(`%s and %s`, matches[4], querySuffix),
	}, nil
}

func toLegacyQuery(q string) (Query, error) {
	matches := legacyRawQueryRegex.FindStringSubmatch(q)

	if len(matches) != 4 {
		return Query{}, errors.Wrapf(ErrInvalidQuery, "query:%s", q)
	}

	return Query{
		Job:       matches[1],
		Group:     matches[2],
		NodePool:  nodePoolDefault,
		MetaQuery: fmt.Sprintf(`%s and %s`, matches[3], querySuffix),
	}, nil
}
