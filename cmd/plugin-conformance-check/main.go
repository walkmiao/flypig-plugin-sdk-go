package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
	pluginruntime "github.com/walkmiao/flypig-plugin-sdk-go/runtime"
)

func main() {
	binary := flag.String("plugin", "", "path to a host-native plugin binary")
	expectedCode := flag.String("expected-code", "", "expected plugin code")
	expectedVersion := flag.String("expected-version", "", "expected plugin SemVer")
	configPath := flag.String("config", "", "JSON config used by lifecycle conformance checks")
	configSchemaVersion := flag.Uint("config-schema-version", 0, "expected config Schema version")
	timeout := flag.Duration("timeout", 20*time.Second, "overall conformance timeout")
	flag.Parse()

	if *binary == "" || *expectedCode == "" || *expectedVersion == "" || *configPath == "" || *configSchemaVersion == 0 {
		fatalf("--plugin, --expected-code, --expected-version, --config, and --config-schema-version are required")
	}
	configJSON, err := os.ReadFile(*configPath)
	if err != nil {
		fatalf("read conformance config: %v", err)
	}
	var configObject map[string]interface{}
	if err := json.Unmarshal(configJSON, &configObject); err != nil {
		fatalf("conformance config must be a JSON object: %v", err)
	}

	client, err := pluginruntime.Dial(*binary)
	if err != nil {
		fatalf("connect plugin: %v", err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	handshake, err := client.API.Handshake(ctx, &pluginapi.HandshakeRequest{
		SupportedPluginApiRange: ">=1.1.0 <2.0.0",
		PlatformVersion:         "developer-kit",
		OperatingSystem:         runtime.GOOS,
		Architecture:            runtime.GOARCH,
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
	mustRPC("Handshake", err)
	mustOK("Handshake", handshake.GetResult())
	if handshake.GetSelectedPluginApiVersion() != pluginruntime.SelectedPluginAPIVersion {
		fatalf("Handshake selected Plugin API %q, expected %q", handshake.GetSelectedPluginApiVersion(), pluginruntime.SelectedPluginAPIVersion)
	}
	if strings.TrimSpace(handshake.GetRuntimeEpoch()) == "" {
		fatalf("Handshake returned an empty runtime epoch")
	}
	ensureCoreCapabilities(handshake.GetCapabilities())
	ensureConnectionSemantics("Handshake", handshake.GetConnectionSemantics())

	info, err := client.API.Info(ctx, &pluginapi.InfoRequest{})
	mustRPC("Info", err)
	mustOK("Info", info.GetResult())
	if info.GetPluginCode() != *expectedCode {
		fatalf("Info pluginCode=%q, expected %q", info.GetPluginCode(), *expectedCode)
	}
	if info.GetVersions() == nil || info.GetVersions().GetPluginVersion() != *expectedVersion {
		fatalf("Info pluginVersion=%q, expected %q", info.GetVersions().GetPluginVersion(), *expectedVersion)
	}
	if info.GetVersions().GetPluginApiVersion() != pluginruntime.SelectedPluginAPIVersion {
		fatalf("Info Plugin API version=%q, expected %q", info.GetVersions().GetPluginApiVersion(), pluginruntime.SelectedPluginAPIVersion)
	}
	if info.GetVersions().GetConfigSchemaVersion() != uint32(*configSchemaVersion) {
		fatalf("Info config Schema version=%d, expected %d", info.GetVersions().GetConfigSchemaVersion(), *configSchemaVersion)
	}
	ensureCoreCapabilities(info.GetCapabilities())
	ensureConnectionSemantics("Info", info.GetConnectionSemantics())
	if !sameConnectionSemantics(handshake.GetConnectionSemantics(), info.GetConnectionSemantics()) {
		fatalf("Handshake and Info connection semantics differ")
	}
	verifyDiagnostics(ctx, client, info.GetCapabilities())

	config := &pluginapi.PluginConfig{SchemaVersion: uint32(*configSchemaVersion), Json: configJSON}
	validated, err := client.API.ValidateConfig(ctx, &pluginapi.ValidateConfigRequest{Config: config})
	mustRPC("ValidateConfig", err)
	mustOK("ValidateConfig", validated.GetResult())
	if validated.GetNormalizedSchemaVersion() != uint32(*configSchemaVersion) {
		fatalf("ValidateConfig normalized Schema version=%d, expected %d", validated.GetNormalizedSchemaVersion(), *configSchemaVersion)
	}

	initialized, err := client.API.Init(ctx, &pluginapi.InitRequest{Config: config})
	mustRPC("Init", err)
	mustLifecycle("Init", initialized, pluginapi.PLUGIN_LIFECYCLE_STATE_VALIDATED)
	started, err := client.API.Start(ctx, &pluginapi.StartRequest{})
	mustRPC("Start", err)
	mustLifecycle("Start", started, pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING)

	health, err := client.API.Health(ctx, &pluginapi.HealthRequest{})
	mustRPC("Health", err)
	mustOK("Health", health.GetResult())
	if !health.GetLive() || !health.GetReady() || health.GetState() != pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING {
		fatalf("Health must be live, ready, and running after Start: %+v", health)
	}
	status, err := client.API.Status(ctx, &pluginapi.StatusRequest{})
	mustRPC("Status", err)
	mustOK("Status", status.GetResult())
	if status.GetState() != pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING || status.GetRuntimeEpoch() != handshake.GetRuntimeEpoch() {
		fatalf("Status state/epoch is inconsistent with Handshake and Start: %+v", status)
	}
	for _, task := range status.GetTasks() {
		validateTask(task)
	}

	stopped, err := client.API.Stop(ctx, &pluginapi.StopRequest{Reason: "developer-kit-conformance"})
	mustRPC("Stop", err)
	mustLifecycle("Stop", stopped, pluginapi.PLUGIN_LIFECYCLE_STATE_STOPPED)
	shutdown, err := client.API.Shutdown(ctx, &pluginapi.ShutdownRequest{Reason: "developer-kit-conformance"})
	mustRPC("Shutdown", err)
	mustLifecycle("Shutdown", shutdown, pluginapi.PLUGIN_LIFECYCLE_STATE_STOPPED)

	fmt.Printf("Plugin API v1 conformance passed: code=%s version=%s\n", *expectedCode, *expectedVersion)
}

func mustRPC(method string, err error) {
	if err != nil {
		fatalf("%s RPC failed: %v", method, err)
	}
}

func mustOK(method string, result *pluginapi.OperationResult) {
	if result == nil || !result.GetOk() {
		fatalf("%s returned failure: %+v", method, result)
	}
}

func mustLifecycle(method string, response *pluginapi.LifecycleResponse, expected pluginapi.PluginLifecycleState) {
	if response == nil {
		fatalf("%s returned a nil response", method)
	}
	mustOK(method, response.GetResult())
	if response.GetState() != expected || strings.TrimSpace(response.GetRuntimeEpoch()) == "" {
		fatalf("%s lifecycle response is invalid: %+v", method, response)
	}
}

func ensureCoreCapabilities(capabilities []pluginapi.Capability) {
	seen := make(map[pluginapi.Capability]struct{}, len(capabilities))
	for _, capability := range capabilities {
		seen[capability] = struct{}{}
	}
	for _, required := range []pluginapi.Capability{
		pluginapi.CAPABILITY_HEALTH,
		pluginapi.CAPABILITY_STATUS,
		pluginapi.CAPABILITY_TASK_MANAGEMENT,
		pluginapi.CAPABILITY_TELEMETRY,
	} {
		if _, ok := seen[required]; !ok {
			fatalf("required capability is missing: %s", required.String())
		}
	}
}

func ensureConnectionSemantics(method string, values []pluginapi.ConnectionSemantics) {
	if len(values) == 0 {
		fatalf("%s returned no connection semantics", method)
	}
	for _, value := range values {
		switch value {
		case pluginapi.CONNECTION_SEMANTICS_SESSION, pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE, pluginapi.CONNECTION_SEMANTICS_LISTENER:
		default:
			fatalf("%s returned unsupported connection semantics %s", method, value.String())
		}
	}
}

func sameConnectionSemantics(left, right []pluginapi.ConnectionSemantics) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[pluginapi.ConnectionSemantics]int, len(left))
	for _, value := range left {
		seen[value]++
	}
	for _, value := range right {
		seen[value]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

func verifyDiagnostics(ctx context.Context, client *pluginruntime.Client, capabilities []pluginapi.Capability) {
	seen := make(map[pluginapi.Capability]struct{}, len(capabilities))
	for _, capability := range capabilities {
		seen[capability] = struct{}{}
	}
	needsDiagnostics := false
	for _, capability := range []pluginapi.Capability{pluginapi.CAPABILITY_INTERACTION_LOG, pluginapi.CAPABILITY_METRICS, pluginapi.CAPABILITY_DIAGNOSTICS} {
		if _, ok := seen[capability]; ok {
			needsDiagnostics = true
		}
	}
	if !needsDiagnostics {
		return
	}
	if client.Diagnostics == nil {
		fatalf("diagnostics capability declared but diagnostics service is unavailable")
	}
	if _, ok := seen[pluginapi.CAPABILITY_INTERACTION_LOG]; ok {
		response, err := client.Diagnostics.ListInteractionLogs(ctx, &pluginapi.ListInteractionLogsRequest{PageSize: 10, IncludeRawPayload: false})
		mustRPC("ListInteractionLogs", err)
		mustOK("ListInteractionLogs", response.GetResult())
		for _, entry := range response.GetEntries() {
			if len(entry.GetRawPayload()) != 0 {
				fatalf("ListInteractionLogs returned raw payload when includeRawPayload=false")
			}
			ensureNoSensitiveDiagnosticKeys("interaction attributes", entry.GetAttributes())
		}
	}
	if _, ok := seen[pluginapi.CAPABILITY_METRICS]; ok {
		response, err := client.Diagnostics.GetMetricsSnapshot(ctx, &pluginapi.GetMetricsSnapshotRequest{})
		mustRPC("GetMetricsSnapshot", err)
		mustOK("GetMetricsSnapshot", response.GetResult())
		for _, sample := range response.GetSamples() {
			ensureNoSensitiveDiagnosticKeys("metric labels", sample.GetLabels())
		}
	}
	if _, ok := seen[pluginapi.CAPABILITY_DIAGNOSTICS]; ok {
		response, err := client.Diagnostics.GetDiagnosticSnapshot(ctx, &pluginapi.GetDiagnosticSnapshotRequest{})
		mustRPC("GetDiagnosticSnapshot", err)
		mustOK("GetDiagnosticSnapshot", response.GetResult())
		if response.GetSnapshot() != nil {
			ensureNoSensitiveDiagnosticKeys("diagnostic attributes", response.GetSnapshot().GetAttributes())
		}
	}
}

func ensureNoSensitiveDiagnosticKeys(scope string, values map[string]string) {
	for key := range values {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
		for _, forbidden := range []string{"password", "secret", "token", "community", "privatekey", "credential", "authkey", "privkey"} {
			if strings.Contains(normalized, forbidden) {
				fatalf("%s contains a sensitive key: %s", scope, key)
			}
		}
	}
}

func validateTask(task *pluginapi.TaskRuntime) {
	if task == nil {
		fatalf("Status contains a nil task")
	}
	switch task.GetConnectionSemantics() {
	case pluginapi.CONNECTION_SEMANTICS_SESSION, pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE, pluginapi.CONNECTION_SEMANTICS_LISTENER:
	default:
		fatalf("task %s has unspecified or unsupported connection semantics", task.GetTaskId())
	}
	if task.GetReady() && !task.GetConnected() {
		fatalf("task %s reports ready=true while connected=false", task.GetTaskId())
	}
	if task.GetState() == pluginapi.TASK_STATE_RUNNING && (!task.GetConnected() || !task.GetReady()) {
		fatalf("task %s reports running without connected=true and ready=true", task.GetTaskId())
	}
	if task.GetConnectionState() == pluginapi.CONNECTION_STATE_UP && !task.GetConnected() {
		fatalf("task %s reports connectionState=UP while connected=false", task.GetTaskId())
	}
	if task.GetConnectionSemantics() == pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE {
		window, err := ptypes.Duration(task.GetConnectionHealthWindow())
		if err != nil || window <= 0 {
			fatalf("task %s request-response semantics requires a positive health window", task.GetTaskId())
		}
		if task.GetConnected() {
			lastSuccess, err := ptypes.Timestamp(task.GetLastSuccessAt())
			if err != nil || lastSuccess.IsZero() {
				fatalf("task %s connected request-response status requires lastSuccessAt", task.GetTaskId())
			}
			if time.Since(lastSuccess) > window {
				fatalf("task %s lastSuccessAt is outside the declared health window", task.GetTaskId())
			}
		}
	}
	switch task.GetState() {
	case pluginapi.TASK_STATE_STOPPED, pluginapi.TASK_STATE_MANUAL_STOPPED, pluginapi.TASK_STATE_FORBIDDEN:
		if task.GetConnected() || task.GetReady() {
			fatalf("task %s remains connected after entering %s", task.GetTaskId(), task.GetState().String())
		}
	case pluginapi.TASK_STATE_FAILED:
		if task.GetConnected() || task.GetReady() || (strings.TrimSpace(task.GetReason()) == "" && strings.TrimSpace(task.GetLastError()) == "") {
			fatalf("task %s has an invalid failed state", task.GetTaskId())
		}
	}
}

func fatalf(format string, values ...interface{}) {
	fmt.Fprintf(os.Stderr, "plugin conformance failed: "+format+"\n", values...)
	os.Exit(1)
}
