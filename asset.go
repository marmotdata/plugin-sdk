package pluginsdk

import "time"

// Asset is a discovered catalog asset.
type Asset struct {
	ParentMRN     *string                `json:"parent_mrn,omitempty"`
	Name          *string                `json:"name,omitempty"`
	Description   *string                `json:"description,omitempty"`
	Type          string                 `json:"type"`
	Providers     []string               `json:"providers"`
	MRN           *string                `json:"mrn,omitempty"`
	Schema        map[string]string      `json:"schema,omitempty"`
	Metadata      map[string]any         `json:"metadata,omitempty"`
	Sources       []AssetSource          `json:"sources,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Environments  map[string]Environment `json:"environments,omitempty"`
	Query         *string                `json:"query,omitempty"`
	QueryLanguage *string                `json:"query_language,omitempty"`
	ExternalLinks []AssetExternalLink    `json:"external_links,omitempty"`
}

// AssetSource records which source contributed an asset's properties.
type AssetSource struct {
	Name       string         `json:"name"`
	LastSyncAt time.Time      `json:"last_sync_at"`
	Properties map[string]any `json:"properties"`
	Priority   int            `json:"priority"`
}

// AssetExternalLink is an external link attached to a single asset.
type AssetExternalLink struct {
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
	URL  string `json:"url"`
}

// Environment describes an asset's presence in a named environment.
type Environment struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
