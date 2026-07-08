package pluginsdk

import "context"

// Source is the interface a Marmot plugin implements. It matches the
// semantics of Marmot's in-process plugin interface: Validate checks a
// raw config and returns it (or an error), Discover performs asset
// discovery with a validated config.
type Source interface {
	Validate(config RawConfig) (RawConfig, error)
	Discover(ctx context.Context, config RawConfig) (*DiscoveryResult, error)
}

// DataFetcher is an optional interface a Source can implement to support
// fetching sample data for asset previews. It matches the semantics of
// Marmot's in-process DataFetcher interface. Plugins whose Source
// implements it advertise the capability via Meta.SupportsDataPreview.
type DataFetcher interface {
	FetchSampleData(ctx context.Context, config RawConfig, a *Asset) (columnNames []string, rows [][]interface{}, err error)
}

// Meta describes a plugin to the Marmot host: identity, display
// information, and the config spec used to render its settings form.
//
// Status and Features drive documentation rendering. Status is one of
// "stable", "beta", or "experimental". Features is the list of asset
// kinds the plugin produces (e.g. "Assets", "Lineage", "Run History").
//
// SupportsDataPreview is set by Serve when the plugin's Source
// implements DataFetcher; plugin authors never set it themselves.
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
