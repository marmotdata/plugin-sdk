package pluginsdk

import (
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// PluginProcess is a handle to a running plugin process, created by the
// Marmot host with Open. Callers must Kill it when done; plugin
// processes are meant to be short-lived (open, call, kill).
type PluginProcess struct {
	client *goplugin.Client
	Source RemoteSource
}

// Open launches the plugin binary at path and connects to it over gRPC.
func Open(path string, logger hclog.Logger) (*PluginProcess, error) {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			PluginSetName: &sourcePlugin{},
		},
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Logger:           logger,
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("connecting to plugin %s: %w", path, err)
	}

	raw, err := rpcClient.Dispense(PluginSetName)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("dispensing source from plugin %s: %w", path, err)
	}

	source, ok := raw.(RemoteSource)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin %s does not implement the source protocol", path)
	}

	return &PluginProcess{client: client, Source: source}, nil
}

// Kill terminates the plugin process.
func (p *PluginProcess) Kill() {
	p.client.Kill()
}
