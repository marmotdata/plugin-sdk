package pluginsdk

import "time"

// AssetRunHistory contains run-history events for an asset.
type AssetRunHistory struct {
	AssetMRN string            `json:"asset_mrn"`
	Runs     []RunHistoryEvent `json:"runs"`
}

// RunHistoryEvent is a single run event.
type RunHistoryEvent struct {
	RunID        string         `json:"run_id"`
	JobNamespace string         `json:"job_namespace"`
	JobName      string         `json:"job_name"`
	EventType    string         `json:"event_type"`
	EventTime    time.Time      `json:"event_time"`
	RunFacets    map[string]any `json:"run_facets,omitempty"`
	JobFacets    map[string]any `json:"job_facets,omitempty"`
}
