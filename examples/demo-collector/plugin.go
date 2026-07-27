package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes"
	duration "github.com/golang/protobuf/ptypes/duration"
	"github.com/walkmiao/flypig-demo-collector/buildinfo"
	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
	pluginruntime "github.com/walkmiao/flypig-plugin-sdk-go/runtime"
)

type demoConfig struct {
	IntervalMilliseconds int `json:"intervalMilliseconds"`
}

type DemoPlugin struct {
	*pluginruntime.BasePlugin

	mu       sync.RWMutex
	tasks    map[string]*pluginapi.TaskRuntime
	sequence atomic.Uint64
}

func NewDemoPlugin() *DemoPlugin {
	buildTime, _ := time.Parse(time.RFC3339, buildinfo.BuildTime)
	return &DemoPlugin{
		BasePlugin: pluginruntime.NewBasePlugin(pluginruntime.Metadata{
			Code:                "demo-collector",
			Name:                "Demo Collector",
			Vendor:              "FlyPig",
			Description:         "Minimal Plugin API v1 collector for SDK and contract testing",
			PluginVersion:       buildinfo.Version,
			PlatformVersion:     "0.8.0",
			ConfigSchemaVersion: 1,
			PointSchemaVersion:  1,
			EventSchemaVersion:  1,
			Language:            "go",
			LanguageVersion:     "go1.24",
			GitCommit:           buildinfo.GitCommit,
			BuildTime:           buildTime,
			ConnectionSemantics: []pluginapi.ConnectionSemantics{pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE},
			Capabilities: []pluginapi.Capability{
				pluginapi.CAPABILITY_HEALTH,
				pluginapi.CAPABILITY_STATUS,
				pluginapi.CAPABILITY_TASK_MANAGEMENT,
				pluginapi.CAPABILITY_TELEMETRY,
				pluginapi.CAPABILITY_DEVICE_DISCOVERY,
				pluginapi.CAPABILITY_POINT_DISCOVERY,
			},
		}),
		tasks: make(map[string]*pluginapi.TaskRuntime),
	}
}

func (p *DemoPlugin) ValidateConfig(_ context.Context, request *pluginapi.ValidateConfigRequest) (*pluginapi.ValidateConfigResponse, error) {
	if request == nil || request.Config == nil {
		return &pluginapi.ValidateConfigResponse{
			Result:     pluginruntime.Failure(pluginapi.ERROR_CODE_INVALID_CONFIG, "config is required", false),
			Violations: []*pluginapi.FieldViolation{{FieldPath: "$", Code: "required", Message: "config is required"}},
		}, nil
	}
	config := demoConfig{IntervalMilliseconds: 1000}
	if len(request.Config.Json) > 0 {
		if err := json.Unmarshal(request.Config.Json, &config); err != nil {
			return &pluginapi.ValidateConfigResponse{
				Result:     pluginruntime.Failure(pluginapi.ERROR_CODE_INVALID_CONFIG, "config JSON is invalid", false),
				Violations: []*pluginapi.FieldViolation{{FieldPath: "$", Code: "json_invalid", Message: err.Error()}},
			}, nil
		}
	}
	if config.IntervalMilliseconds < 100 {
		return &pluginapi.ValidateConfigResponse{
			Result: pluginruntime.Failure(pluginapi.ERROR_CODE_INVALID_CONFIG, "intervalMilliseconds must be at least 100", false),
			Violations: []*pluginapi.FieldViolation{{
				FieldPath: "intervalMilliseconds",
				Code:      "minimum",
				Message:   "must be at least 100",
			}},
		}, nil
	}
	return &pluginapi.ValidateConfigResponse{Result: pluginruntime.Success(), NormalizedSchemaVersion: 1}, nil
}

func (p *DemoPlugin) ApplyTasks(_ context.Context, request *pluginapi.ApplyTasksRequest) (*pluginapi.ApplyTasksResponse, error) {
	response := &pluginapi.ApplyTasksResponse{}
	if request == nil {
		return response, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, task := range request.Tasks {
		if task == nil || task.TaskId == "" {
			continue
		}
		now, _ := ptypes.TimestampProto(time.Now().UTC())
		state := pluginapi.TASK_STATE_STOPPED
		connectionState := pluginapi.CONNECTION_STATE_DOWN
		connected := false
		ready := false
		reason := "task disabled"
		if task.Enabled {
			state = pluginapi.TASK_STATE_RUNNING
			connectionState = pluginapi.CONNECTION_STATE_UP
			connected = true
			ready = true
			reason = ""
		}
		statusRevision := task.Revision
		if statusRevision == 0 {
			statusRevision = 1
		}
		runtime := &pluginapi.TaskRuntime{
			TaskId:                 task.TaskId,
			Protocol:               task.Protocol,
			ConnectionSemantics:    pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE,
			ConnectionHealthWindow: &duration.Duration{Seconds: 30},
			AppliedRevision:        task.Revision,
			State:                  state,
			ConnectionState:        connectionState,
			Enabled:                task.Enabled,
			Connected:              connected,
			Ready:                  ready,
			Reason:                 reason,
			Endpoint:               "demo://local",
			UpdatedAt:              now,
			Epoch:                  p.RuntimeEpoch(),
			StatusRevision:         statusRevision,
		}
		if connected {
			runtime.LastSuccessAt = now
		}
		p.tasks[task.TaskId] = runtime
		response.Results = append(response.Results, &pluginapi.TaskResult{
			TaskId:   task.TaskId,
			Revision: task.Revision,
			Result:   pluginruntime.Success(),
		})
	}
	return response, nil
}

func (p *DemoPlugin) StopTasks(_ context.Context, request *pluginapi.StopTasksRequest) (*pluginapi.StopTasksResponse, error) {
	response := &pluginapi.StopTasksResponse{}
	if request == nil {
		return response, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, taskID := range request.TaskIds {
		task := p.tasks[taskID]
		if task == nil {
			response.Results = append(response.Results, &pluginapi.TaskResult{
				TaskId: taskID,
				Result: pluginruntime.Failure(pluginapi.ERROR_CODE_TASK_NOT_FOUND, "task not found", false),
			})
			continue
		}
		now, _ := ptypes.TimestampProto(time.Now().UTC())
		task.State = pluginapi.TASK_STATE_STOPPED
		task.ConnectionState = pluginapi.CONNECTION_STATE_DOWN
		task.Connected = false
		task.Ready = false
		task.Reason = request.Reason
		task.UpdatedAt = now
		task.StatusRevision++
		response.Results = append(response.Results, &pluginapi.TaskResult{TaskId: taskID, Revision: task.AppliedRevision, Result: pluginruntime.Success()})
	}
	return response, nil
}

func (p *DemoPlugin) ListTasks(context.Context, *pluginapi.ListTasksRequest) (*pluginapi.ListTasksResponse, error) {
	return &pluginapi.ListTasksResponse{Result: pluginruntime.Success(), Tasks: p.taskSnapshot()}, nil
}

func (p *DemoPlugin) Status(context.Context, *pluginapi.StatusRequest) (*pluginapi.StatusResponse, error) {
	now, _ := ptypes.TimestampProto(time.Now().UTC())
	return &pluginapi.StatusResponse{
		Result:           pluginruntime.Success(),
		State:            p.LifecycleState(),
		RuntimeEpoch:     p.RuntimeEpoch(),
		SnapshotRevision: p.sequence.Load() + 1,
		Tasks:            p.taskSnapshot(),
		At:               now,
	}, nil
}

func (p *DemoPlugin) DiscoverDevices(_ context.Context, request *pluginapi.DiscoverDevicesRequest) (*pluginapi.DiscoverDevicesResponse, error) {
	taskID := ""
	if request != nil {
		taskID = request.GetTaskId()
	}
	return &pluginapi.DiscoverDevicesResponse{
		Result: pluginruntime.Success(),
		Devices: []*pluginapi.Device{{
			DeviceId:   "demo-device-1",
			ExternalId: "demo-device-1",
			Name:       "Demo Device",
			Type:       "simulator",
			Labels:     map[string]string{"taskId": taskID},
			Address:    map[string]string{"scheme": "demo"},
		}},
		Page: &pluginapi.PageInfo{},
	}, nil
}

func (p *DemoPlugin) DiscoverPoints(_ context.Context, request *pluginapi.DiscoverPointsRequest) (*pluginapi.DiscoverPointsResponse, error) {
	deviceID := ""
	if request != nil {
		deviceID = request.GetDeviceId()
	}
	if deviceID == "" {
		deviceID = "demo-device-1"
	}
	return &pluginapi.DiscoverPointsResponse{
		Result: pluginruntime.Success(),
		Points: []*pluginapi.Point{
			{PointId: "temperature", PointKey: "temperature", DeviceId: deviceID, Name: "Temperature", ValueType: "double", Unit: "Cel", Address: map[string]string{"index": "1"}},
			{PointId: "running", PointKey: "running", DeviceId: deviceID, Name: "Running", ValueType: "bool", Address: map[string]string{"index": "2"}},
		},
		Page: &pluginapi.PageInfo{},
	}, nil
}

func (p *DemoPlugin) Collect(_ context.Context, request *pluginapi.CollectRequest) (*pluginapi.CollectResponse, error) {
	if request == nil || request.TaskId == "" {
		return &pluginapi.CollectResponse{Result: pluginruntime.Failure(pluginapi.ERROR_CODE_INVALID_ARGUMENT, "taskId is required", false)}, nil
	}
	p.mu.RLock()
	task := p.tasks[request.TaskId]
	p.mu.RUnlock()
	if task == nil {
		return &pluginapi.CollectResponse{Result: pluginruntime.Failure(pluginapi.ERROR_CODE_TASK_NOT_FOUND, "task not found", false)}, nil
	}
	now := time.Now().UTC()
	timestamp, _ := ptypes.TimestampProto(now)
	temperature := 20.0 + float64(now.Unix()%100)/10
	return &pluginapi.CollectResponse{
		Result: pluginruntime.Success(),
		Values: []*pluginapi.TelemetryValue{
			{
				TaskId: request.TaskId, DeviceId: "demo-device-1", PointId: "temperature", PointKey: "temperature",
				Timestamp: timestamp, SourceTimestamp: timestamp, ReceivedAt: timestamp,
				RawValue:         &pluginapi.Value{Kind: &pluginapi.Value_DoubleValue{DoubleValue: temperature}},
				EngineeringValue: &pluginapi.Value{Kind: &pluginapi.Value_DoubleValue{DoubleValue: temperature}},
				Unit:             "Cel", Quality: &pluginapi.Quality{Level: pluginapi.QUALITY_LEVEL_GOOD},
				Source: &pluginapi.SourceContext{PluginCode: "demo-collector", PluginVersion: buildinfo.Version, TaskId: request.TaskId},
			},
			{
				TaskId: request.TaskId, DeviceId: "demo-device-1", PointId: "running", PointKey: "running",
				Timestamp: timestamp, SourceTimestamp: timestamp, ReceivedAt: timestamp,
				RawValue:         &pluginapi.Value{Kind: &pluginapi.Value_BoolValue{BoolValue: true}},
				EngineeringValue: &pluginapi.Value{Kind: &pluginapi.Value_BoolValue{BoolValue: true}},
				Quality:          &pluginapi.Quality{Level: pluginapi.QUALITY_LEVEL_GOOD},
				Source:           &pluginapi.SourceContext{PluginCode: "demo-collector", PluginVersion: buildinfo.Version, TaskId: request.TaskId},
			},
		},
	}, nil
}

func (p *DemoPlugin) StreamEvents(_ *pluginapi.StreamEventsRequest, stream pluginapi.CollectorPluginService_StreamEventsServer) error {
	interval := time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			tasks := p.taskSnapshot()
			if len(tasks) == 0 {
				continue
			}
			collected, _ := p.Collect(stream.Context(), &pluginapi.CollectRequest{TaskId: tasks[0].TaskId})
			sequence := p.sequence.Add(1)
			events := make([]*pluginapi.PluginEvent, 0, len(collected.Values))
			for index, value := range collected.Values {
				events = append(events, &pluginapi.PluginEvent{
					EventId: fmt.Sprintf("demo-%d-%d", sequence, index),
					Event:   &pluginapi.PluginEvent_Telemetry{Telemetry: value},
				})
			}
			if err := stream.Send(&pluginapi.EventBatch{
				BatchId: fmt.Sprintf("demo-%d", sequence), Epoch: p.RuntimeEpoch(),
				SequenceStart: sequence, SequenceEnd: sequence, Events: events,
			}); err != nil {
				return err
			}
		}
	}
}

func (p *DemoPlugin) taskSnapshot() []*pluginapi.TaskRuntime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]string, 0, len(p.tasks))
	for id := range p.tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]*pluginapi.TaskRuntime, 0, len(ids))
	for _, id := range ids {
		copy := *p.tasks[id]
		result = append(result, &copy)
	}
	return result
}
