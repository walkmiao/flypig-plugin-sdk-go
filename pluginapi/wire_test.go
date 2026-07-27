package pluginapi

import (
	"testing"

	proto "github.com/golang/protobuf/proto"
)

func TestCanonicalProtoFilesRegistered(t *testing.T) {
	for _, name := range []string{
		"api/plugin/v1/common.proto",
		"api/plugin/v1/events.proto",
		"api/plugin/v1/discovery.proto",
		"api/plugin/v1/command.proto",
		"api/plugin/v1/diagnostics.proto",
		"api/plugin/v1/plugin.proto",
	} {
		if descriptor := proto.FileDescriptor(name); len(descriptor) == 0 {
			t.Fatalf("canonical protobuf file is not registered: %s", name)
		}
	}
}

func TestCanonicalMessagesRoundTripOnWire(t *testing.T) {
	messages := []proto.Message{
		&OperationResult{
			Ok: true,
			Error: &ErrorDetail{
				Code:    ERROR_CODE_INTERNAL,
				Message: "wire-test",
				Details: map[string]string{"scope": "common"},
			},
		},
		&PluginEvent{
			EventId: "event-1",
			Event: &PluginEvent_Telemetry{Telemetry: &TelemetryValue{
				TaskId:           "task-1",
				PointId:          "point-1",
				EngineeringValue: &Value{Kind: &Value_StringValue{StringValue: "42"}},
			}},
		},
		&Device{
			DeviceId: "device-1",
			Labels:   map[string]string{"site": "test"},
		},
		&ExecuteCommandRequest{
			CommandId:  "command-1",
			TaskId:     "task-1",
			Operation:  "interrogation",
			Attributes: map[string]string{"source": "wire-test"},
		},
		&ListInteractionLogsRequest{TaskId: "task-1", PageSize: 10},
		&HandshakeRequest{
			SupportedPluginApiRange: ">=1.1.0 <2.0.0",
			PlatformVersion:         "wire-test",
			OperatingSystem:         "test",
			Architecture:            "test",
			Limits:                  &HostLimits{MaxUnaryMessageBytes: 1024},
			SupportedConnectionSemantics: []ConnectionSemantics{
				CONNECTION_SEMANTICS_SESSION,
			},
		},
	}

	for _, message := range messages {
		encoded, err := proto.Marshal(message)
		if err != nil {
			t.Fatalf("marshal %T: %v", message, err)
		}
		decoded := proto.Clone(message)
		decoded.Reset()
		if err := proto.Unmarshal(encoded, decoded); err != nil {
			t.Fatalf("unmarshal %T: %v", message, err)
		}
		if !proto.Equal(message, decoded) {
			t.Fatalf("wire round trip differs for %T: before=%v after=%v", message, message, decoded)
		}
	}
}
