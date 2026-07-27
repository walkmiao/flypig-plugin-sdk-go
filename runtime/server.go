package runtime

import (
	"context"
	"fmt"

	plugin "github.com/hashicorp/go-plugin"
	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
	"google.golang.org/grpc"
)

const (
	PluginName       = "collector"
	ProtocolVersion  = 1
	MagicCookieKey   = "FLYPIG_PLUGIN_MAGIC_COOKIE"
	MagicCookieValue = "flypig-collector-plugin-v1"
)

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  ProtocolVersion,
	MagicCookieKey:   MagicCookieKey,
	MagicCookieValue: MagicCookieValue,
}

type serviceClients struct {
	pluginapi.CollectorPluginServiceClient
	Diagnostics pluginapi.PluginDiagnosticsServiceClient
}

type grpcPlugin struct {
	plugin.Plugin
	server pluginapi.CollectorPluginServiceServer
}

func (p *grpcPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	if p.server == nil {
		return fmt.Errorf("collector plugin server is nil")
	}
	pluginapi.RegisterCollectorPluginServiceServer(server, p.server)
	if diagnostics, ok := p.server.(pluginapi.PluginDiagnosticsServiceServer); ok {
		pluginapi.RegisterPluginDiagnosticsServiceServer(server, diagnostics)
	}
	return nil
}

func (p *grpcPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, conn *grpc.ClientConn) (interface{}, error) {
	return &serviceClients{
		CollectorPluginServiceClient: pluginapi.NewCollectorPluginServiceClient(conn),
		Diagnostics:                  pluginapi.NewPluginDiagnosticsServiceClient(conn),
	}, nil
}

// Serve starts the standard HashiCorp go-plugin process and registers the
// canonical FlyPig Plugin API v1 services. Business plugins should call Serve
// from main and must not implement the handshake protocol themselves.
func Serve(server pluginapi.CollectorPluginServiceServer) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: HandshakeConfig,
		VersionedPlugins: map[int]plugin.PluginSet{
			ProtocolVersion: {
				PluginName: &grpcPlugin{server: server},
			},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}
