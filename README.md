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

If your `Source` also implements `DataFetcher`, Marmot can show sample rows on asset pages. It gets the asset and the plugin config, queries the source system, and returns column names and rows. `Serve` picks this up automatically, there is nothing to register.

The host spawns a fresh plugin process for every call, so `Discover` never shares an instance with an earlier `Validate` call. The SDK runs `Validate` before `Discover` in each process, so state your `Validate` sets on the `Source` (parsed config, computed limits) is always there when `Discover` runs.

Use `ApplyDefaults` in `Validate` to fill fields from their `default:"..."` tags when the key is absent from the raw config, the same tags the settings form uses. Without it, defaults only apply to configs created through the Marmot UI.

The `plugintest` package tests your built binary over the real wire protocol:

```go
func TestDiscover(t *testing.T) {
    bin := plugintest.Build(t, "..") // plugin main package
    result, err := bin.Discover(context.Background(), pluginsdk.RawConfig{...})
    ...
}
```

Build the binary with a `marmot-plugin-` name prefix and drop it in `~/.marmot/plugins` (or the directory set by `MARMOT_PLUGINS_DIR`). Marmot discovers it at startup, fetches its metadata, and registers it alongside the built-in plugins. Plugin processes are short-lived: the host spawns the binary per call and kills it when the call completes.

See [marmot-plugin-gcs](https://github.com/marmotdata/marmot-plugin-gcs) for a complete real-world plugin.

## The packages

| Package | Contents |
| --- | --- |
| `pluginsdk` | `Source` and `Meta`, the plugin-facing types (`Asset`, `DiscoveryResult`, `LineageEdge`, ...), config helpers (`UnmarshalConfig`, `ValidateStruct`, `GenerateConfigSpec`, `InterpolateTags`), `Serve` for plugin binaries, and `Open` for hosts |
| `mrn` | Build and parse Marmot Resource Names (`mrn://bucket/gcs/my-bucket`) |
| `proto` | The gRPC wire protocol (`GetMeta`/`Validate`/`Discover`/`FetchSampleData`); payloads are JSON so the protocol stays stable while types evolve |

## Config specs

`GenerateConfigSpec` reflects over your config struct's tags to produce the settings form Marmot renders in its UI:

- `json` — field name
- `description` — help text
- `label` — display label (defaults to a title-cased field name)
- `validate` — `required` marks the field required; `oneof=a b c` renders a dropdown
- `sensitive:"true"` — renders as a password field and masks the value
- `default` — default value
- `show_when:"field:value"` — conditional visibility

`BaseConfig` (embed it inline) adds the standard `tags`, `external_links`, and `filter` fields every plugin supports. Filtering is applied by the host after discovery; plugins only carry the config.

## Protocol

The handshake pins `ProtocolVersion` (currently 1). Bump it on breaking changes to the wire protocol; hosts refuse plugins built against a different version. Regenerate the gRPC code after editing `proto/plugin.proto`:

```sh
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/plugin.proto
```
