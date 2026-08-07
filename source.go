package pluginsdk

import "context"

// Source is the interface a Marmot plugin implements. Validate checks a
// raw config and returns it (or an error); Discover runs discovery with
// a validated config.
type Source interface {
	Validate(config RawConfig) (RawConfig, error)
	Discover(ctx context.Context, config RawConfig) (*DiscoveryResult, error)
}

// DataFetcher is an optional Source extension that supplies sample data
// for asset previews. Plugins whose Source implements it are surfaced
// via Meta.SupportsDataPreview.
type DataFetcher interface {
	FetchSampleData(ctx context.Context, config RawConfig, a *Asset) (columnNames []string, rows [][]any, err error)
}

// Meta describes a plugin to the Marmot host: identity, display
// information, and the config spec used to render its settings form.
// Status is one of "stable", "beta", or "experimental". Features lists
// the asset kinds the plugin produces (e.g. "Assets", "Lineage", "Run
// History"). SupportsDataPreview is set by Serve when the plugin's
// Source implements DataFetcher; plugin authors never set it.
type Meta struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	Description         string        `json:"description"`
	Icon                string        `json:"icon"`
	Category            string        `json:"category"`
	Status              string        `json:"status"`
	Features            []string      `json:"features,omitempty"`
	ConfigSpec          []ConfigField `json:"config_spec"`
	SupportsDataPreview bool          `json:"supports_data_preview,omitempty"`
}
