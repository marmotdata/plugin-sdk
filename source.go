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

// Meta describes a plugin to the Marmot host: identity, display
// information, and the config spec used to render its settings form.
type Meta struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Icon        string        `json:"icon"`
	Category    string        `json:"category"`
	ConfigSpec  []ConfigField `json:"config_spec"`
}
