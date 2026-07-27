package testkit

import (
	"context"
	"net"
	"testing"
	"time"

	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufferSize = 1024 * 1024

// RunConformance exercises the mandatory Plugin API v1 lifecycle against an
// in-memory gRPC server. Protocol-specific plugins should add their own tests
// for task, discovery, telemetry, command, and event behavior.
func RunConformance(t *testing.T, server pluginapi.CollectorPluginServiceServer, expectedCode string) {
	t.Helper()
	listener := bufconn.Listen(bufferSize)
	grpcServer := grpc.NewServer()
	pluginapi.RegisterCollectorPluginServiceServer(grpcServer, server)
	if diagnostics, ok := server.(pluginapi.PluginDiagnosticsServiceServer); ok {
		pluginapi.RegisterPluginDiagnosticsServiceServer(grpcServer, diagnostics)
	}
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(grpcServer.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "bufconn", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial in-memory plugin: %v", err)
	}
	defer conn.Close()
	client := pluginapi.NewCollectorPluginServiceClient(conn)

	handshake, err := client.Handshake(ctx, &pluginapi.HandshakeRequest{
		SupportedPluginApiRange: ">=1.1.0 <2.0.0",
		PlatformVersion:         "developer-kit-test",
		SupportedConnectionSemantics: []pluginapi.ConnectionSemantics{
			pluginapi.CONNECTION_SEMANTICS_SESSION,
			pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE,
			pluginapi.CONNECTION_SEMANTICS_LISTENER,
		},
		Limits: &pluginapi.HostLimits{
			MaxUnaryMessageBytes: 4 * 1024 * 1024,
			MaxEventBatchBytes:   4 * 1024 * 1024,
			MaxEventBatchItems:   1000,
			MaxInflightRequests:  16,
		},
	})
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if handshake.Result == nil || !handshake.Result.Ok || handshake.SelectedPluginApiVersion != "1.1.0" || handshake.RuntimeEpoch == "" {
		t.Fatalf("invalid handshake response: %+v", handshake)
	}
	info, err := client.Info(ctx, &pluginapi.InfoRequest{})
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Result == nil || !info.Result.Ok || info.PluginCode != expectedCode || info.Versions == nil || info.Versions.PluginApiVersion != "1.1.0" {
		t.Fatalf("invalid info response: %+v", info)
	}
	if len(info.ConnectionSemantics) == 0 || len(handshake.ConnectionSemantics) == 0 {
		t.Fatalf("plugin must declare connection semantics: handshake=%+v info=%+v", handshake.ConnectionSemantics, info.ConnectionSemantics)
	}
	validation, err := client.ValidateConfig(ctx, &pluginapi.ValidateConfigRequest{Config: &pluginapi.PluginConfig{SchemaVersion: info.Versions.ConfigSchemaVersion, Json: []byte(`{}`)}})
	if err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
	if validation.Result == nil || !validation.Result.Ok {
		t.Fatalf("config validation failed: %+v", validation)
	}
	initialized, err := client.Init(ctx, &pluginapi.InitRequest{Config: &pluginapi.PluginConfig{SchemaVersion: info.Versions.ConfigSchemaVersion, Json: []byte(`{}`)}})
	if err != nil || initialized.State != pluginapi.PLUGIN_LIFECYCLE_STATE_VALIDATED {
		t.Fatalf("Init response=%+v err=%v", initialized, err)
	}
	started, err := client.Start(ctx, &pluginapi.StartRequest{})
	if err != nil || started.State != pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING {
		t.Fatalf("Start response=%+v err=%v", started, err)
	}
	health, err := client.Health(ctx, &pluginapi.HealthRequest{})
	if err != nil || !health.Live || !health.Ready || health.State != pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING {
		t.Fatalf("Health response=%+v err=%v", health, err)
	}
	status, err := client.Status(ctx, &pluginapi.StatusRequest{})
	if err != nil || status.RuntimeEpoch == "" || status.State != pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING {
		t.Fatalf("Status response=%+v err=%v", status, err)
	}
	stopped, err := client.Stop(ctx, &pluginapi.StopRequest{Reason: "conformance-test"})
	if err != nil || stopped.State != pluginapi.PLUGIN_LIFECYCLE_STATE_STOPPED {
		t.Fatalf("Stop response=%+v err=%v", stopped, err)
	}
	shutdown, err := client.Shutdown(ctx, &pluginapi.ShutdownRequest{Reason: "conformance-test"})
	if err != nil || shutdown.State != pluginapi.PLUGIN_LIFECYCLE_STATE_STOPPED {
		t.Fatalf("Shutdown response=%+v err=%v", shutdown, err)
	}
}
