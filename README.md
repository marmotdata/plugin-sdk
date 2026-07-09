# plugin-sdk

SDK for building external [Marmot](https://github.com/marmotdata/marmot) plugins.

Marmot plugins are standalone binaries that the Marmot host launches on demand via [go-plugin](https://github.com/hashicorp/go-plugin) and talks to over gRPC. This SDK provides everything both sides need: the wire protocol, the plugin-facing types and helpers, and the host-side client.

## Writing a plugin

A plugin implements the `Source` interface and hands it to `Serve` in its main function:

```go
package main

import (
	"context"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

type Config struct {
	pluginsdk.BaseConfig `json:",inline"`

	ProjectID string `json:"project_id" description:"Project to discover" validate:"required"`
}

type Source struct{}

func (s *Source) Validate(raw pluginsdk.RawConfig) (pluginsdk.RawConfig, error) {
	config, err := pluginsdk.UnmarshalConfig[Config](raw)
	if err != nil {
		return nil, err
	}
	pluginsdk.ApplyDefaults(config, raw)
	if err := pluginsdk.ValidateStruct(config); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Source) Discover(ctx context.Context, raw pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	// discover assets, lineage, and documentation here
	return &pluginsdk.DiscoveryResult{}, nil
}

func main() {
	pluginsdk.Serve(&pluginsdk.ServeConfig{
		Meta: pluginsdk.Meta{
			ID:          "example",
			Name:        "Example",
			Description: "Discovers things from Example",
			Icon:        "example",
			Category:    "storage",
			ConfigSpec:  pluginsdk.GenerateConfigSpec(Config{}),
		},
		Source: &Source{},
	})
}
```

Build the binary with a `marmot-plugin-` name prefix and drop it in `~/.marmot/plugins` (or the directory set by `MARMOT_PLUGINS_DIR`). Marmot discovers it at startup, fetches its metadata, and registers it; a local binary shadows a downloaded core plugin with the same ID. Plugin processes are short-lived: the host spawns the binary per call and kills it, and the SDK runs `Validate` before `Discover` in each process, so state your `Validate` sets on the `Source` is there when `Discover` runs.

A `Source` can optionally implement `DataFetcher` to power sample-row previews on asset pages; `Serve` detects it automatically. Test your built binary over the real wire protocol with the `plugintest` package.

See [marmot-plugin-gcs](https://github.com/marmotdata/marmot-plugin-gcs) for a complete real-world plugin, and [Creating a Marmot Plugin](https://marmotdata.io/docs/next/Develop/creating-plugins) for a step-by-step guide.

## The packages

| Package | Contents |
| --- | --- |
| `pluginsdk` | `Source`, `DataFetcher`, and `Meta`; the plugin-facing types (`Asset`, `DiscoveryResult`, `LineageEdge`, ...); config helpers (`UnmarshalConfig`, `ApplyDefaults`, `ValidateStruct`, `GenerateConfigSpec`, `InterpolateTags`); AWS helpers; `Serve` for plugin binaries and `Open` for hosts |
| `filesource` | Resolve file paths from local disk, `s3://bucket/prefix`, or `git::` URLs into a local directory |
| `mrn` | Build and parse Marmot Resource Names (`mrn://bucket/gcs/my-bucket`) |
| `plugintest` | End-to-end test helpers that run a built plugin binary over the wire protocol |
| `proto` | The gRPC wire protocol (`GetMeta`/`Validate`/`Discover`/`FetchSampleData`); payloads are JSON so the protocol stays stable while types evolve |

## Config specs

`GenerateConfigSpec` reflects over your config struct's tags to produce the settings form Marmot renders in its UI:

- `json`: field name
- `description`: help text
- `label`: display label (defaults to a title-cased field name)
- `validate`: `required` marks the field required; `oneof=a b c` renders a dropdown
- `sensitive:"true"`: renders as a password field and masks the value
- `default`: default value
- `show_when:"field:value"`: conditional visibility

The `default` tag pre-fills the UI form; call `ApplyDefaults(config, raw)` in `Validate` so defaults also apply to configs written by hand.

`BaseConfig` (embed it inline) adds the standard `tags`, `external_links`, and `filter` fields every plugin supports. Filtering is applied by the host after discovery; plugins only carry the config.

## Protocol

The handshake pins `ProtocolVersion` (currently 1). Bump it on breaking changes to the wire protocol; hosts refuse plugins built against a different version. Adding an RPC is not breaking: old plugins answer it with `Unimplemented`. Regenerate the gRPC code after editing `proto/plugin.proto`:

```sh
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/plugin.proto
```
