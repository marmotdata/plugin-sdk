package pluginsdk

// LineageEdge is a lineage relationship between two assets.
type LineageEdge struct {
	Source        string       `json:"source"`
	Target        string       `json:"target"`
	Type          string       `json:"type"`
	JobMRN        string       `json:"job_mrn,omitempty"`
	ColumnLineage []ColumnEdge `json:"column_lineage,omitempty"`
}

// ColumnEdge is one entry of a LineageEdge's ColumnLineage: it records which
// columns of the edge's source asset were used to produce one column of the
// edge's target asset.
//
// For example, an edge from "postgres://raw.users" to
// "postgres://mart.customers" could carry a ColumnEdge with FromColumns
// ["first_name", "last_name"] and ToColumn "full_name", meaning
// mart.customers.full_name is computed from those two raw.users columns.
type ColumnEdge struct {
	// FromColumns are the source-asset columns the target column is derived
	// from. More than one entry means the target column combines them.
	FromColumns []string `json:"from_columns"`

	// ToColumn is the target-asset column being produced.
	ToColumn string `json:"to_column"`

	// Transform describes how FromColumns become ToColumn, e.g. "expression"
	// or a SQL snippet. It is free-form text shown to users as-is and never
	// parsed; leave it empty for plain column copies.
	Transform string `json:"transform,omitempty"`

	// Confidence is how certain the producer is of this mapping, from 0 to 1.
	// Zero is omitted from the JSON and so cannot be told apart from unset;
	// it is therefore read as full confidence (1.0). Only set it when
	// reporting a mapping below full confidence.
	Confidence float32 `json:"confidence,omitempty"`
}
