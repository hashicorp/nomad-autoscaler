package nomadmeta

import (
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/pkg/node"
	"github.com/hashicorp/nomad/api"
	"github.com/pkg/errors"
)

type NodeCounter interface {
	GetNodeNames(filter, nodePool string) ([]string, error)
	RunningOnIneligibleNodes(job string) ([]string, error)
}

var _ NodeCounter = (*nodeCounter)(nil)

type nodeCounter struct {
	client   *api.Nodes
	jobs     *api.Jobs
	nodes    node.Status
	logger   hclog.Logger
	pageSize int32
}

func (n *nodeCounter) RunningOnIneligibleNodes(job string) ([]string, error) {
	allocs, _, err := n.jobs.Allocations(job, true, &api.QueryOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "fail to list allocations")
	}

	var nodes []string

	for _, alloc := range allocs {
		if alloc.DesiredStatus != "run" {
			continue
		}
		if n.nodes.IsIneligible(alloc.NodeID) {
			nodes = append(nodes, alloc.NodeName)
			n.logger.Info("job has allocation running on ineligible node", "node", alloc.NodeName, "job", job)
		}
	}

	return nodes, nil
}

func NewCounter(client *api.Client, logger hclog.Logger, nodes node.Status, pageSize int) *nodeCounter {
	return &nodeCounter{
		client:   client.Nodes(),
		jobs:     client.Jobs(),
		nodes:    nodes,
		logger:   logger,
		pageSize: int32(pageSize),
	}
}

func (n *nodeCounter) GetNodeNames(filter, nodePool string) ([]string, error) {
	token := ""
	var names []string

	for {
		batchNames, nextToken, err := n.list(filter, nodePool, token)
		if err != nil {
			return names, errors.Wrapf(err, "fail to list nodes")
		}

		names = append(names, batchNames...)
		token = nextToken

		if nextToken == "" {
			break
		}
	}

	return names, nil
}

func (n *nodeCounter) list(filter, nodePool, token string) ([]string, string, error) {
	nodeList, meta, err := n.client.List(&api.QueryOptions{
		Filter:     filter,
		AllowStale: false,
		PerPage:    n.pageSize,
		NextToken:  token,
	})

	if err != nil {
		return nil, "", err
	}

	var nodes []string

	for _, nd := range nodeList {
		if nd.Status == "down" {
			continue
		}
		if nd.NodePool == nodePool || nodePool == nodePoolAllNodes {
			nodes = append(nodes, nd.Name)
		}
	}

	return nodes, meta.NextToken, nil
}
