package pluginsdk

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
)

const DumpMetadataFlag = "--dump-metadata"

// ServeConfig holds everything a plugin binary needs to serve itself to
// the Marmot host.
type ServeConfig struct {
	Meta   Meta
	Source Source
}

// Serve hands the plugin over to go-plugin and blocks until the host
// disconnects. Call it from the plugin's main function:
//
//	func main() {
//	    pluginsdk.Serve(&pluginsdk.ServeConfig{
//	        Meta:   meta,
//	        Source: &Source{},
//	    })
//	}
func Serve(config *ServeConfig) {
	if len(os.Args) > 1 && os.Args[1] == DumpMetadataFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(config.Meta); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:       config.Meta.ID,
		Level:      hclog.Info,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			PluginSetName: &sourcePlugin{meta: config.Meta, source: config.Source},
		},
		GRPCServer: goplugin.DefaultGRPCServer,
		Logger:     logger,
	})
}
