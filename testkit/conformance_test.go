package testkit

import (
	"testing"

	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
	pluginruntime "github.com/walkmiao/flypig-plugin-sdk-go/runtime"
)

func TestRunConformanceSupportsStandardConnectionSemantics(t *testing.T) {
	tests := []struct {
		name      string
		semantics pluginapi.ConnectionSemantics
	}{
		{name: "session", semantics: pluginapi.CONNECTION_SEMANTICS_SESSION},
		{name: "request-response", semantics: pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE},
		{name: "listener", semantics: pluginapi.CONNECTION_SEMANTICS_LISTENER},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code := "testkit-" + test.name
			plugin := pluginruntime.NewBasePlugin(pluginruntime.Metadata{
				Code:                code,
				Name:                code,
				Vendor:              "FlyPig",
				PluginVersion:       "0.0.0-test",
				PlatformVersion:     "0.8.0",
				ConfigSchemaVersion: 1,
				PointSchemaVersion:  1,
				EventSchemaVersion:  1,
				ConnectionSemantics: []pluginapi.ConnectionSemantics{test.semantics},
			})

			RunConformance(t, plugin, code)
		})
	}
}
