package pluginsdk

// LineageEdge is a lineage relationship between two assets.
type LineageEdge struct {
	Source        string       `json:"source"`
	Target        string       `json:"target"`
	Type          string       `json:"type"`
	JobMRN        string       `json:"job_mrn,omitempty"`
	ColumnLineage []ColumnEdge `json:"column_lineage,omitempty"`
}

// ColumnEdge maps source columns to a target column; Transform is display-only free-form text and Confidence is 0..1 with zero treated as 1.0.
type ColumnEdge struct {
	FromColumns []string `json:"from_columns"`
	ToColumn    string   `json:"to_column"`
	Transform   string   `json:"transform,omitempty"`
	Confidence  float32  `json:"confidence,omitempty"`
}
