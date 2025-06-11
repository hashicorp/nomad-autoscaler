package main

import (
	"strconv"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad-autoscaler/pkg/node"
	"github.com/hashicorp/nomad-autoscaler/pkg/nomadmeta"
	"github.com/hashicorp/nomad-autoscaler/plugins"
	"github.com/hashicorp/nomad-autoscaler/plugins/apm"
	"github.com/hashicorp/nomad-autoscaler/plugins/base"
	"github.com/hashicorp/nomad-autoscaler/sdk"
	"github.com/hashicorp/nomad/api"
	"github.com/pkg/errors"
)

const (
	pluginName = "nomad-meta-apm-v2"
)

var (
	pluginInfo = &base.PluginInfo{
		Name:       pluginName,
		PluginType: sdk.PluginTypeAPM,
	}
)

var _ apm.APM = (*NomadMeta)(nil)

type NomadMeta struct {
	logger          hclog.Logger
	client          *nomadmeta.NomadMeta
	nodeWatcherLock sync.RWMutex
	nodeWatcher     *node.NodeStatusWatcher
}

func (n *NomadMeta) SetConfig(config map[string]string) error {
	n.logger.Debug("set config", "config", config)

	nomadAddress, ok := config[nomadmeta.ConfigKeyNomadAddress]

	if !ok {
		return errors.New("nomad_address is a required config attribute")
	}

	client, err := api.NewClient(&api.Config{Address: nomadAddress})
	if err != nil {
		return errors.Wrap(err, "could not create a nomad api client")
	}

	pageSize := nomadmeta.DefaultPageSize

	pageSizeCfg, ok := config[nomadmeta.ConfigKeyPageSize]
	if ok {
		val, err := strconv.Atoi(pageSizeCfg)
		if err != nil {
			n.logger.Error("config key error, value must be a number, setting the default value", "key", nomadmeta.ConfigKeyPageSize)
		} else {
			pageSize = val
		}
	}

	n.nodeWatcherLock.Lock()
	defer n.nodeWatcherLock.Unlock()

	// Start node watcher loop if not started
	if n.nodeWatcher == nil {
		n.nodeWatcher, err = node.NewNodeStatusWatcher(client, n.logger)
		if err != nil {
			return errors.Wrapf(err, "could not initialize node status watcher")
		}
	} else { // refresh client
		n.nodeWatcher.SetClient(client)
	}

	n.client = nomadmeta.NewNomadMeta(n.logger, nomadmeta.NewCounter(client, n.logger, n.nodeWatcher, pageSize))

	return nil
}

func (n *NomadMeta) Query(q string, r sdk.TimeRange) (sdk.TimestampedMetrics, error) {
	query, err := nomadmeta.ToQuery(q)
	if err != nil {
		return nil, err
	}
	return n.client.Query(query, r)
}

func (n *NomadMeta) PluginInfo() (*base.PluginInfo, error) {
	return pluginInfo, nil
}

func (n *NomadMeta) QueryMultiple(q string, r sdk.TimeRange) ([]sdk.TimestampedMetrics, error) {
	m, err := n.Query(q, r)
	if err != nil {
		return nil, err
	}

	return []sdk.TimestampedMetrics{m}, nil
}

func main() {
	plugins.Serve(factory)
}

func factory(l hclog.Logger) interface{} {
	return &NomadMeta{logger: l}
}
