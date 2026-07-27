package runtime

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	plugin "github.com/hashicorp/go-plugin"
	pluginapi "github.com/walkmiao/flypig-plugin-sdk-go/pluginapi"
)

// Client owns one HashiCorp go-plugin process and exposes the canonical
// Plugin API v1 clients. Call Close when the plugin process is no longer needed.
type Client struct {
	process     *plugin.Client
	API         pluginapi.CollectorPluginServiceClient
	Diagnostics pluginapi.PluginDiagnosticsServiceClient
}

// DialOptions controls the host-side process environment. Business plugins do
// not need this type; it is intended for Data Plane supervisors and test hosts.
type DialOptions struct {
	// Dir becomes the plugin process working directory. Leave empty to inherit
	// the host working directory.
	Dir string
	// Env is the complete child environment. Nil inherits os.Environ; an empty
	// non-nil slice intentionally starts with no inherited variables.
	Env []string
	// Stdout and Stderr receive plugin process output. Nil uses the host streams.
	Stdout io.Writer
	Stderr io.Writer
}

// Dial starts one plugin binary with the standard FlyPig handshake and
// connects to its Plugin API v1 services. It preserves the historical behavior
// of inheriting the host environment and current working directory.
func Dial(binary string) (*Client, error) {
	return DialWithOptions(binary, DialOptions{Env: os.Environ(), Stdout: os.Stdout, Stderr: os.Stderr})
}

// DialWithOptions starts a plugin with an explicit process boundary. The
// HashiCorp go-plugin handshake variables are still injected by go-plugin.
func DialWithOptions(binary string, options DialOptions) (*Client, error) {
	if binary == "" {
		return nil, fmt.Errorf("plugin binary path is required")
	}
	cmd := exec.Command(binary)
	cmd.Dir = options.Dir
	if options.Env == nil {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = append([]string(nil), options.Env...)
	}
	stdout := options.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := options.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	process := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: HandshakeConfig,
		VersionedPlugins: map[int]plugin.PluginSet{
			ProtocolVersion: {
				PluginName: &grpcPlugin{},
			},
		},
		Cmd:              cmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		SyncStdout:       stdout,
		SyncStderr:       stderr,
	})
	rpcClient, err := process.Client()
	if err != nil {
		process.Kill()
		return nil, fmt.Errorf("start plugin process: %w", err)
	}
	raw, err := rpcClient.Dispense(PluginName)
	if err != nil {
		process.Kill()
		return nil, fmt.Errorf("dispense plugin service: %w", err)
	}
	api, ok := raw.(pluginapi.CollectorPluginServiceClient)
	if !ok {
		process.Kill()
		return nil, fmt.Errorf("unexpected plugin client type %T", raw)
	}
	client := &Client{process: process, API: api}
	if services, ok := raw.(*serviceClients); ok {
		client.Diagnostics = services.Diagnostics
	}
	return client, nil
}

// Close terminates the managed plugin process.
func (c *Client) Close() {
	if c == nil || c.process == nil {
		return
	}
	c.process.Kill()
	c.process = nil
	c.API = nil
	c.Diagnostics = nil
}
