// Package pluginsdk is the SDK for building external Marmot plugins.
//
// Plugins are standalone binaries that Marmot launches on demand via
// go-plugin and talks to over gRPC. A plugin implements the
// Source interface and hands it to Serve in its main function.
//
// The types in this package mirror the JSON shapes of Marmot's core
// types (assets, lineage, documentation), so results cross the process
// boundary as JSON without plugins importing Marmot internals.
package pluginsdk

import "time"

// RawConfig holds the raw configuration for a plugin run. It uses a
// map[string]interface{} to carry arbitrary user-provided YAML/JSON.
type RawConfig map[string]interface{}

// BaseConfig holds the config fields shared by every plugin.
// Embed it inline in your plugin's Config struct:
//
//	type Config struct {
//	    pluginsdk.BaseConfig `json:",inline"`
//	    ...
//	}
type BaseConfig struct {
	Tags          TagsConfig     `json:"tags,omitempty" description:"Tags to apply to discovered assets"`
	ExternalLinks []ExternalLink `json:"external_links,omitempty" description:"External links to show on all assets"`
	Filter        *Filter        `json:"filter,omitempty" description:"Filter discovered assets by name (regex)"`
}

// Filter filters discovered assets by name. Filtering is applied by the
// Marmot host after discovery; plugins only need to carry the field.
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

// Asset is a discovered catalog asset. It mirrors Marmot's core asset
// JSON shape.
type Asset struct {
	ParentMRN     *string                `json:"parent_mrn,omitempty"`
	Name          *string                `json:"name,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Type          string                 `json:"type"`
	Providers     []string               `json:"providers"`
	MRN           *string                `json:"mrn,omitempty"`
	Schema        map[string]string      `json:"schema,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Sources       []AssetSource          `json:"sources,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Environments  map[string]Environment `json:"environments,omitempty"`
	Query         *string                `json:"query,omitempty"`
	QueryLanguage *string                `json:"query_language,omitempty"`
	ExternalLinks []AssetExternalLink    `json:"external_links,omitempty"`
}

// AssetSource records which source contributed an asset's properties.
type AssetSource struct {
	Name       string                 `json:"name"`
	LastSyncAt time.Time              `json:"last_sync_at"`
	Properties map[string]interface{} `json:"properties"`
	Priority   int                    `json:"priority"`
}

// AssetExternalLink is an external link attached to a single asset.
type AssetExternalLink struct {
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
	URL  string `json:"url"`
}

// Environment describes an asset's presence in a named environment.
type Environment struct {
	Name     string                 `json:"name"`
	Path     string                 `json:"path"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// LineageEdge is a lineage relationship between two assets.
type LineageEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	JobMRN string `json:"job_mrn,omitempty"`
}

// Documentation is markdown documentation attached to an asset.
type Documentation struct {
	MRN     string `json:"mrn"`
	Content string `json:"content"`
	Source  string `json:"source"`
}

// Statistic is a single metric value for an asset.
type Statistic struct {
	AssetMRN   string  `json:"asset_mrn"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
}

// AssetRunHistory contains run history events for an asset.
type AssetRunHistory struct {
	AssetMRN string            `json:"asset_mrn"`
	Runs     []RunHistoryEvent `json:"runs"`
}

// RunHistoryEvent represents a single run event (START, COMPLETE, FAIL, etc.)
type RunHistoryEvent struct {
	RunID        string                 `json:"run_id"`
	JobNamespace string                 `json:"job_namespace"`
	JobName      string                 `json:"job_name"`
	EventType    string                 `json:"event_type"`
	EventTime    time.Time              `json:"event_time"`
	RunFacets    map[string]interface{} `json:"run_facets,omitempty"`
	JobFacets    map[string]interface{} `json:"job_facets,omitempty"`
}

// DiscoveryResult contains everything a plugin discovered in one run.
type DiscoveryResult struct {
	Assets        []Asset           `json:"assets"`
	Lineage       []LineageEdge     `json:"lineage"`
	Documentation []Documentation   `json:"documentation"`
	Statistics    []Statistic       `json:"statistics"`
	RunHistory    []AssetRunHistory `json:"run_history,omitempty"`
}
