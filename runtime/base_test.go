package runtime

import (
	"context"
	"testing"

	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
)

func TestVersionRangeContainsSelectedPluginAPI(t *testing.T) {
	tests := []struct {
		rangeText string
		want      bool
	}{
		{rangeText: ">=1.1.0 <2.0.0", want: true},
		{rangeText: ">=1.0.0 <2.0.0", want: true},
		{rangeText: "1.1.0", want: true},
		{rangeText: ">=1.0.0 <1.1.0", want: false},
		{rangeText: ">=2.0.0 <3.0.0", want: false},
		{rangeText: "", want: false},
	}
	for _, test := range tests {
		if got := versionRangeContains(test.rangeText, SelectedPluginAPIVersion); got != test.want {
			t.Fatalf("versionRangeContains(%q) = %v, want %v", test.rangeText, got, test.want)
		}
	}
}

func TestHandshakeRequiresCompatibleConnectionSemantics(t *testing.T) {
	plugin := NewBasePlugin(Metadata{
		Code:                "demo",
		Name:                "Demo",
		Vendor:              "FlyPig",
		PluginVersion:       "1.0.0",
		ConnectionSemantics: []pluginapi.ConnectionSemantics{pluginapi.CONNECTION_SEMANTICS_SESSION},
	})
	response, err := plugin.Handshake(context.Background(), &pluginapi.HandshakeRequest{
		SupportedPluginApiRange:      ">=1.1.0 <2.0.0",
		SupportedConnectionSemantics: []pluginapi.ConnectionSemantics{pluginapi.CONNECTION_SEMANTICS_REQUEST_RESPONSE},
	})
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if response.GetResult().GetOk() || response.GetResult().GetError().GetCode() != pluginapi.ERROR_CODE_UNSUPPORTED_CAPABILITY {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestHandshakeSelectsPluginAPI11(t *testing.T) {
	plugin := NewBasePlugin(Metadata{
		Code:                "demo",
		Name:                "Demo",
		Vendor:              "FlyPig",
		PluginVersion:       "1.0.0",
		ConnectionSemantics: []pluginapi.ConnectionSemantics{pluginapi.CONNECTION_SEMANTICS_SESSION},
	})
	response, err := plugin.Handshake(context.Background(), &pluginapi.HandshakeRequest{
		SupportedPluginApiRange:      ">=1.0.0 <2.0.0",
		SupportedConnectionSemantics: []pluginapi.ConnectionSemantics{pluginapi.CONNECTION_SEMANTICS_SESSION},
	})
	if err != nil {
		t.Fatalf("Handshake: %v", err)
	}
	if !response.GetResult().GetOk() || response.GetSelectedPluginApiVersion() != "1.1.0" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
