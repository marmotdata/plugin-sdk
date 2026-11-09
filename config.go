package pluginsdk

import (
	"fmt"

	"github.com/marmotdata/plugin-sdk/mrn"

	"sigs.k8s.io/yaml"
)

// RawConfig is a plugin's configuration as it arrives from the Marmot
// host: a decoded YAML or JSON object, keyed by field name, awaiting
// unmarshalling into the plugin's typed config.
type RawConfig map[string]any

// BaseConfig holds the config fields shared by every plugin. Embed it
// inline in your plugin's Config struct:
//
//	type Config struct {
//	    pluginsdk.BaseConfig `json:",inline"`
//	    ...
//	}
type BaseConfig struct {
	Tags          TagsConfig     `json:"tags,omitempty" description:"Tags to apply to discovered assets"`
	ExternalLinks []ExternalLink `json:"external_links,omitempty" description:"External links to show on all assets"`
	Filter        *Filter        `json:"filter,omitempty" description:"Filter discovered assets by name (regex)"`
	// Zone tells apart two systems of the same technology, such as two
	// PostgreSQL servers that hold identical databases. It becomes part of
	// every MRN the plugin builds with AssetMRN. Empty is the default
	// zone, which is what a catalog built before zones existed used.
	Zone string `json:"zone,omitempty" group:"Discovery" description:"Tells this system apart from others of the same technology and becomes part of every asset's MRN"`
}

// ZoneKey is the raw config key holding a plugin's zone.
const ZoneKey = "zone"

// AssetMRN builds the MRN of an asset this plugin owns, carrying the
// configured zone. Path is the asset's place in the system's own
// hierarchy, outermost first, for example database, schema and table.
func (b BaseConfig) AssetMRN(assetType, provider string, path ...string) string {
	return mrn.Asset(assetType, provider, path...).WithZone(b.Zone).String()
}

// RawZone returns the zone set in a raw config, or "".
func RawZone(config RawConfig) string {
	zone, _ := config[ZoneKey].(string)
	return zone
}

// Filter narrows discovered assets by name. The Marmot host applies it
// after discovery; plugins only need to carry the field.
type Filter struct {
	Include []string `json:"include,omitempty" description:"Include patterns for resource names (regex)"`
	Exclude []string `json:"exclude,omitempty" description:"Exclude patterns for resource names (regex)"`
}

// ExternalLink defines an external resource link.
type ExternalLink struct {
	Name string `json:"name" description:"Display name for the link" validate:"required"`
	Icon string `json:"icon,omitempty" description:"Icon identifier for the link"`
	URL  string `json:"url" description:"URL to the external resource" validate:"required,url"`
}

// UnmarshalConfig decodes a RawConfig into a typed plugin config T.
func UnmarshalConfig[T any](raw RawConfig) (*T, error) {
	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-marshalling config: %w", err)
	}

	var config T
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshalling into plugin config: %w", err)
	}

	return &config, nil
}
