package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes"
	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
)

const SelectedPluginAPIVersion = "1.1.0"

type Metadata struct {
	Code                string
	Name                string
	Vendor              string
	Description         string
	PluginVersion       string
	PlatformVersion     string
	ConfigSchemaVersion uint32
	PointSchemaVersion  uint32
	EventSchemaVersion  uint32
	Language            string
	LanguageVersion     string
	GitCommit           string
	BuildTime           time.Time
	Capabilities        []pluginapi.Capability
	ConnectionSemantics []pluginapi.ConnectionSemantics
}

// BasePlugin implements the required Plugin API v1 lifecycle and safe defaults.
// Embed it in a plugin implementation and override protocol-specific methods.
type BasePlugin struct {
	pluginapi.UnimplementedCollectorPluginServiceServer
	pluginapi.UnimplementedPluginDiagnosticsServiceServer

	mu       sync.RWMutex
	metadata Metadata
	state    pluginapi.PluginLifecycleState
	epoch    string
	started  time.Time
}

func NewBasePlugin(metadata Metadata) *BasePlugin {
	if metadata.BuildTime.IsZero() {
		metadata.BuildTime = time.Now().UTC()
	}
	return &BasePlugin{
		metadata: metadata,
		state:    pluginapi.PLUGIN_LIFECYCLE_STATE_INSTALLED,
		epoch:    newEpoch(),
	}
}

func (p *BasePlugin) Metadata() Metadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

func (p *BasePlugin) RuntimeEpoch() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.epoch
}

func (p *BasePlugin) LifecycleState() pluginapi.PluginLifecycleState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

func (p *BasePlugin) SetLifecycleState(state pluginapi.PluginLifecycleState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state = state
	if state == pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING && p.started.IsZero() {
		p.started = time.Now().UTC()
	}
}

func (p *BasePlugin) Handshake(_ context.Context, request *pluginapi.HandshakeRequest) (*pluginapi.HandshakeResponse, error) {
	if request == nil || !versionRangeContains(request.SupportedPluginApiRange, SelectedPluginAPIVersion) {
		return &pluginapi.HandshakeResponse{
			Result: Failure(pluginapi.ERROR_CODE_UNSUPPORTED_API_VERSION, "host does not allow Plugin API "+SelectedPluginAPIVersion, false),
		}, nil
	}
	metadata := p.Metadata()
	if len(metadata.ConnectionSemantics) == 0 {
		return &pluginapi.HandshakeResponse{
			Result: Failure(pluginapi.ERROR_CODE_INVALID_CONFIG, "plugin must declare at least one connection semantics", false),
		}, nil
	}
	if len(request.SupportedConnectionSemantics) == 0 || !connectionSemanticsOverlap(metadata.ConnectionSemantics, request.SupportedConnectionSemantics) {
		return &pluginapi.HandshakeResponse{
			Result: Failure(pluginapi.ERROR_CODE_UNSUPPORTED_CAPABILITY, "host and plugin have no compatible connection semantics", false),
		}, nil
	}
	return &pluginapi.HandshakeResponse{
		Result:                   Success(),
		SelectedPluginApiVersion: SelectedPluginAPIVersion,
		Versions:                 versionSet(metadata),
		Capabilities:             append([]pluginapi.Capability(nil), metadata.Capabilities...),
		EffectiveLimits:          request.Limits,
		RuntimeEpoch:             p.RuntimeEpoch(),
		ConnectionSemantics:      append([]pluginapi.ConnectionSemantics(nil), metadata.ConnectionSemantics...),
	}, nil
}

func (p *BasePlugin) Info(context.Context, *pluginapi.InfoRequest) (*pluginapi.InfoResponse, error) {
	metadata := p.Metadata()
	buildTime, _ := ptypes.TimestampProto(metadata.BuildTime.UTC())
	return &pluginapi.InfoResponse{
		Result:              Success(),
		PluginCode:          metadata.Code,
		PluginName:          metadata.Name,
		Vendor:              metadata.Vendor,
		Description:         metadata.Description,
		Versions:            versionSet(metadata),
		Language:            metadata.Language,
		LanguageVersion:     metadata.LanguageVersion,
		GitCommit:           metadata.GitCommit,
		BuildTime:           buildTime,
		Capabilities:        append([]pluginapi.Capability(nil), metadata.Capabilities...),
		ConnectionSemantics: append([]pluginapi.ConnectionSemantics(nil), metadata.ConnectionSemantics...),
	}, nil
}

func (p *BasePlugin) ValidateConfig(context.Context, *pluginapi.ValidateConfigRequest) (*pluginapi.ValidateConfigResponse, error) {
	metadata := p.Metadata()
	return &pluginapi.ValidateConfigResponse{
		Result:                  Success(),
		NormalizedSchemaVersion: metadata.ConfigSchemaVersion,
	}, nil
}

func (p *BasePlugin) Init(context.Context, *pluginapi.InitRequest) (*pluginapi.LifecycleResponse, error) {
	p.SetLifecycleState(pluginapi.PLUGIN_LIFECYCLE_STATE_VALIDATED)
	return p.lifecycleResponse(), nil
}

func (p *BasePlugin) Start(context.Context, *pluginapi.StartRequest) (*pluginapi.LifecycleResponse, error) {
	p.SetLifecycleState(pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING)
	return p.lifecycleResponse(), nil
}

func (p *BasePlugin) Stop(context.Context, *pluginapi.StopRequest) (*pluginapi.LifecycleResponse, error) {
	p.SetLifecycleState(pluginapi.PLUGIN_LIFECYCLE_STATE_STOPPED)
	return p.lifecycleResponse(), nil
}

func (p *BasePlugin) Reload(ctx context.Context, request *pluginapi.ReloadRequest) (*pluginapi.LifecycleResponse, error) {
	validation, err := p.ValidateConfig(ctx, &pluginapi.ValidateConfigRequest{Context: request.Context, Config: request.Config})
	if err != nil {
		return nil, err
	}
	if validation.Result == nil || !validation.Result.Ok {
		return &pluginapi.LifecycleResponse{Result: validation.Result, State: p.LifecycleState(), RuntimeEpoch: p.RuntimeEpoch()}, nil
	}
	return p.lifecycleResponse(), nil
}

func (p *BasePlugin) Shutdown(context.Context, *pluginapi.ShutdownRequest) (*pluginapi.LifecycleResponse, error) {
	p.SetLifecycleState(pluginapi.PLUGIN_LIFECYCLE_STATE_STOPPED)
	return p.lifecycleResponse(), nil
}

func (p *BasePlugin) Health(context.Context, *pluginapi.HealthRequest) (*pluginapi.HealthResponse, error) {
	state := p.LifecycleState()
	now, _ := ptypes.TimestampProto(time.Now().UTC())
	return &pluginapi.HealthResponse{
		Result:      Success(),
		Live:        true,
		Ready:       state == pluginapi.PLUGIN_LIFECYCLE_STATE_RUNNING,
		State:       state,
		HeartbeatAt: now,
	}, nil
}

func (p *BasePlugin) Status(context.Context, *pluginapi.StatusRequest) (*pluginapi.StatusResponse, error) {
	now, _ := ptypes.TimestampProto(time.Now().UTC())
	return &pluginapi.StatusResponse{
		Result:           Success(),
		State:            p.LifecycleState(),
		RuntimeEpoch:     p.RuntimeEpoch(),
		SnapshotRevision: 1,
		At:               now,
	}, nil
}

func (p *BasePlugin) ApplyTasks(context.Context, *pluginapi.ApplyTasksRequest) (*pluginapi.ApplyTasksResponse, error) {
	return &pluginapi.ApplyTasksResponse{}, nil
}

func (p *BasePlugin) StopTasks(context.Context, *pluginapi.StopTasksRequest) (*pluginapi.StopTasksResponse, error) {
	return &pluginapi.StopTasksResponse{}, nil
}

func (p *BasePlugin) ListTasks(context.Context, *pluginapi.ListTasksRequest) (*pluginapi.ListTasksResponse, error) {
	return &pluginapi.ListTasksResponse{Result: Success()}, nil
}

func (p *BasePlugin) DiscoverDevices(context.Context, *pluginapi.DiscoverDevicesRequest) (*pluginapi.DiscoverDevicesResponse, error) {
	return &pluginapi.DiscoverDevicesResponse{Result: Unsupported("deviceDiscovery")}, nil
}

func (p *BasePlugin) DiscoverPoints(context.Context, *pluginapi.DiscoverPointsRequest) (*pluginapi.DiscoverPointsResponse, error) {
	return &pluginapi.DiscoverPointsResponse{Result: Unsupported("pointDiscovery")}, nil
}

func (p *BasePlugin) Collect(context.Context, *pluginapi.CollectRequest) (*pluginapi.CollectResponse, error) {
	return &pluginapi.CollectResponse{Result: Unsupported("telemetry")}, nil
}

func (p *BasePlugin) ExecuteCommand(context.Context, *pluginapi.ExecuteCommandRequest) (*pluginapi.ExecuteCommandResponse, error) {
	return &pluginapi.ExecuteCommandResponse{Result: Unsupported("command")}, nil
}

func (p *BasePlugin) StreamEvents(request *pluginapi.StreamEventsRequest, stream pluginapi.CollectorPluginService_StreamEventsServer) error {
	<-stream.Context().Done()
	return stream.Context().Err()
}

func (p *BasePlugin) AckEvents(context.Context, *pluginapi.AckEventsRequest) (*pluginapi.AckEventsResponse, error) {
	return &pluginapi.AckEventsResponse{Result: Success()}, nil
}

func (p *BasePlugin) ListInteractionLogs(context.Context, *pluginapi.ListInteractionLogsRequest) (*pluginapi.ListInteractionLogsResponse, error) {
	return &pluginapi.ListInteractionLogsResponse{Result: Unsupported("interactionLog")}, nil
}

func (p *BasePlugin) GetMetricsSnapshot(context.Context, *pluginapi.GetMetricsSnapshotRequest) (*pluginapi.GetMetricsSnapshotResponse, error) {
	return &pluginapi.GetMetricsSnapshotResponse{Result: Unsupported("metrics")}, nil
}

func (p *BasePlugin) GetDiagnosticSnapshot(context.Context, *pluginapi.GetDiagnosticSnapshotRequest) (*pluginapi.GetDiagnosticSnapshotResponse, error) {
	return &pluginapi.GetDiagnosticSnapshotResponse{Result: Unsupported("diagnostics")}, nil
}

func (p *BasePlugin) lifecycleResponse() *pluginapi.LifecycleResponse {
	now, _ := ptypes.TimestampProto(time.Now().UTC())
	return &pluginapi.LifecycleResponse{
		Result:       Success(),
		State:        p.LifecycleState(),
		RuntimeEpoch: p.RuntimeEpoch(),
		At:           now,
	}
}

func versionSet(metadata Metadata) *pluginapi.VersionSet {
	return &pluginapi.VersionSet{
		PlatformVersion:     metadata.PlatformVersion,
		PluginApiVersion:    SelectedPluginAPIVersion,
		PluginVersion:       metadata.PluginVersion,
		ConfigSchemaVersion: metadata.ConfigSchemaVersion,
		PointSchemaVersion:  metadata.PointSchemaVersion,
		EventSchemaVersion:  metadata.EventSchemaVersion,
	}
}

var versionTokenPattern = regexp.MustCompile(`^(>=|<=|>|<|=)?([0-9]+)\.([0-9]+)\.([0-9]+)$`)

func versionRangeContains(value, selected string) bool {
	selectedVersion, ok := parseVersion(selected)
	if !ok {
		return false
	}
	normalized := strings.NewReplacer(",", " ", "&&", " ").Replace(strings.TrimSpace(value))
	if normalized == "" || strings.Contains(normalized, "||") {
		return false
	}
	tokens := strings.Fields(normalized)
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		match := versionTokenPattern.FindStringSubmatch(token)
		if match == nil {
			return false
		}
		candidate, ok := parseVersion(strings.Join(match[2:], "."))
		if !ok {
			return false
		}
		comparison := compareVersion(selectedVersion, candidate)
		switch match[1] {
		case "", "=":
			if comparison != 0 {
				return false
			}
		case ">=":
			if comparison < 0 {
				return false
			}
		case ">":
			if comparison <= 0 {
				return false
			}
		case "<=":
			if comparison > 0 {
				return false
			}
		case "<":
			if comparison >= 0 {
				return false
			}
		}
	}
	return true
}

type semanticVersion [3]int

func parseVersion(value string) (semanticVersion, bool) {
	match := versionTokenPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil || match[1] != "" {
		return semanticVersion{}, false
	}
	var version semanticVersion
	for index, text := range match[2:] {
		value, err := strconv.Atoi(text)
		if err != nil {
			return semanticVersion{}, false
		}
		version[index] = value
	}
	return version, true
}

func compareVersion(left, right semanticVersion) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}

func connectionSemanticsOverlap(pluginValues, hostValues []pluginapi.ConnectionSemantics) bool {
	host := make(map[pluginapi.ConnectionSemantics]struct{}, len(hostValues))
	for _, value := range hostValues {
		if value != pluginapi.CONNECTION_SEMANTICS_UNSPECIFIED {
			host[value] = struct{}{}
		}
	}
	for _, value := range pluginValues {
		if value == pluginapi.CONNECTION_SEMANTICS_UNSPECIFIED {
			continue
		}
		if _, ok := host[value]; ok {
			return true
		}
	}
	return false
}

func newEpoch() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return time.Now().UTC().Format("20060102T150405.000000000")
}
