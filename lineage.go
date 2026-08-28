package pluginsdk

import "context"

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

// ColumnLineageExtractor derives column-level lineage from a SQL query and returns edges keyed by source-table MRN so callers can demux them onto the correct LineageEdge; implementations must be safe for concurrent use.
type ColumnLineageExtractor interface {
	Extract(ctx context.Context, req ExtractRequest) (map[string][]ColumnEdge, error)
}

// ExtractRequest carries the SQL plus the parser context: DefaultDatabase and DefaultSchema seed name resolution for unqualified references and Sources supplies the schemas needed for SELECT * expansion and column disambiguation.
type ExtractRequest struct {
	SQL             string
	Dialect         string
	DefaultDatabase string
	DefaultSchema   string
	Sources         []ExtractSource
}

// ExtractSource is one table the parser may reference; MRN is opaque to the extractor and only used by the caller to attribute derived edges.
type ExtractSource struct {
	MRN      string
	Database string
	Schema   string
	Table    string
	Columns  map[string]string
}
