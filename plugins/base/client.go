// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package base

import (
	"context"
	"fmt"

	"github.com/hashicorp/nomad-autoscaler/plugins/base/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// PluginClient is the gRPC client implementation of the APM interface. It is
// the only exported plugin client as it is consumed and utilised by all other
// plugins.
type PluginClient struct {
	Client  proto.BasePluginServiceClient
	DoneCtx context.Context
}

// PluginInfo is the gRPC client implementation of the Base.PluginInfo
// interface function.
func (p *PluginClient) PluginInfo() (*PluginInfo, error) {
	info, err := p.Client.PluginInfo(p.DoneCtx, &proto.PluginInfoRequest{})
	if err != nil {
		return nil, err
	}

	var pType string
	switch info.GetType() {
	case proto.PluginType_PLUGIN_TYPE_APM:
		pType = sdk.PluginTypeAPM
	case proto.PluginType_PLUGIN_TYPE_STRATEGY:
		pType = sdk.PluginTypeStrategy
	case proto.PluginType_PLUGIN_TYPE_TARGET:
		pType = sdk.PluginTypeTarget
	default:
		return nil, fmt.Errorf("plugin is of unknown type: %q", info.GetType().String())
	}

	return &PluginInfo{
		PluginType: pType,
		Name:       info.GetName(),
	}, nil
}

// SetConfig is the gRPC client implementation of the Base.SetConfig interface
// function.
func (p *PluginClient) SetConfig(cfg map[string]string) error {
	_, err := p.Client.SetConfig(p.DoneCtx, &proto.SetConfigRequest{Config: cfg})
	return err
}
