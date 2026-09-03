package pluginsdk

import (
	"fmt"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// PluginProcess is a handle to a running plugin process, created by the
// Marmot host with Open. Callers must Kill it when done. Discovery hosts
// treat processes as short-lived (open, call, kill) while the data plane
// keeps query-capable processes alive and checks them with Ping.
type PluginProcess struct {
	client   *goplugin.Client
	protocol goplugin.ClientProtocol
	Source   RemoteSource
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

	return &PluginProcess{client: client, protocol: rpcClient, Source: source}, nil
}

// Kill terminates the plugin process.
func (p *PluginProcess) Kill() {
	p.client.Kill()
}

// Ping checks that the plugin process is still alive and serving.
func (p *PluginProcess) Ping() error {
	return p.protocol.Ping()
}

// Exited reports whether the plugin process has terminated.
func (p *PluginProcess) Exited() bool {
	return p.client.Exited()
}
