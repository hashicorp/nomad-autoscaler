package base

import (
	"context"
	"fmt"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad-autoscaler/plugins/base/proto/v1"
	"github.com/hashicorp/nomad-autoscaler/sdk"
)

// pluginServer is the gRPC server implementation of the Base interface.
type pluginServer struct {
	broker *plugin.GRPCBroker
	impl   Base
}

// PluginInfo is the gRPC server implementation of the Base.PluginInfo
// interface function.
func (p *pluginServer) PluginInfo(_ context.Context, _ *proto.PluginInfoRequest) (*proto.PluginInfoResponse, error) {
	info, err := p.impl.PluginInfo()
	if err != nil {
		return nil, err
	}

	var pType proto.PluginType
	switch info.PluginType {
	case sdk.PluginTypeAPM:
		pType = proto.PluginType_PLUGIN_TYPE_APM
	case sdk.PluginTypeStrategy:
		pType = proto.PluginType_PLUGIN_TYPE_STRATEGY
	case sdk.PluginTypeTarget:
		pType = proto.PluginType_PLUGIN_TYPE_TARGET
	default:
		return nil, fmt.Errorf("plugin is of unknown type: %q", info.PluginType)
	}

	return &proto.PluginInfoResponse{
		Type: pType,
		Name: info.Name,
	}, nil
}

// SetConfig is the gRPC server implementation of the Base.SetConfig interface
// function.
func (p *pluginServer) SetConfig(_ context.Context, req *proto.SetConfigRequest) (*proto.SetConfigResponse, error) {
	return &proto.SetConfigResponse{}, p.impl.SetConfig(req.Config)
}
