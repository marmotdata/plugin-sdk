package pluginsdk

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

// PluginProcess is a handle to a running plugin process, created by the
// Marmot host with Open. Callers must Kill it when done. Discovery hosts
// treat processes as short-lived (open, call, kill) while Marmot keeps
// query-capable processes alive and checks them with Ping.
type PluginProcess struct {
	client   *goplugin.Client
	protocol goplugin.ClientProtocol
	Source   RemoteSource
}

// Open launches the plugin binary at path and connects to it over gRPC. The binary must still hash to sha256sum, which the host records when it first trusts the binary, so a binary swapped on disk afterwards is refused. The connection uses mutual TLS with certificates generated for this process alone, and the plugin inherits the host environment minus MARMOT_ variables, which hold the host's own secrets.
func Open(path string, sha256sum []byte, logger hclog.Logger) (*PluginProcess, error) {
	if len(sha256sum) != sha256.Size {
		return nil, fmt.Errorf("opening plugin %s: a sha256 checksum is required", path)
	}
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	cmd := exec.Command(path)
	cmd.Env = pluginEnv(os.Environ())

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			PluginSetName: &sourcePlugin{},
		},
		Cmd:              cmd,
		SkipHostEnv:      true,
		SecureConfig:     &goplugin.SecureConfig{Checksum: sha256sum, Hash: sha256.New()},
		AutoMTLS:         true,
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

// HashBinary returns the sha256 of the plugin binary at path, for passing to Open.
func HashBinary(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening plugin binary: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("hashing plugin binary: %w", err)
	}
	return h.Sum(nil), nil
}

// pluginEnv drops MARMOT_ variables from env: they configure the host, such as its encryption key and database, and a plugin has no business reading them.
func pluginEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "MARMOT_") {
			out = append(out, kv)
		}
	}
	return out
}
