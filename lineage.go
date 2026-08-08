package pluginsdk

// LineageEdge is a lineage relationship between two assets.
type LineageEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	JobMRN string `json:"job_mrn,omitempty"`
}
