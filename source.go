package pluginsdk

import "context"

// Source is the interface a Marmot plugin implements. Validate checks a
// raw config and returns it (or an error); Discover runs discovery with
// a validated config.
type Source interface {
	Validate(config RawConfig) (RawConfig, error)
	Discover(ctx context.Context, config RawConfig) (*DiscoveryResult, error)
}

// DataFetcher is an optional Source extension that supplies sample data
// for asset previews. Plugins whose Source implements it are surfaced
// via Meta.SupportsDataPreview.
type DataFetcher interface {
	FetchSampleData(ctx context.Context, config RawConfig, a *Asset) (columnNames []string, rows [][]any, err error)
}

// Querier is an optional Source extension that executes queries against the
// underlying engine on behalf of the Marmot data plane. PlanQuery asks the
// engine what a statement would touch without running it returning a nil plan
// with a nil error means the engine cannot plan the statement and the host falls
// back to target-level policy. ExecuteQuery runs the statement and pushes result chunks through
// emit; the first chunk carries the column schema and an error returned from
// emit aborts the query. Plugins whose Source implements Querier are surfaced
// via Meta.SupportsQuery.
type Querier interface {
	PlanQuery(ctx context.Context, config RawConfig, req QueryRequest) (*QueryPlan, error)
	ExecuteQuery(ctx context.Context, config RawConfig, req QueryRequest, emit func(chunk QueryResultChunk) error) error
}

// QueryRequest is a single statement the host wants planned or executed.
// MaxRows caps the rows the plugin returns (0 means no cap beyond the
// host's own) and Identity carries the acting principal's name so engines
// that support it can attribute the query.
type QueryRequest struct {
	Statement string `json:"statement"`
	MaxRows   int64  `json:"max_rows,omitempty"`
	Identity  string `json:"identity,omitempty"`
}

// QueryPlan describes what a statement would touch, in MRN terms so the
// host can make policy decisions without understanding engine SQL.
type QueryPlan struct {
	ReferencedMRNs []string `json:"referenced_mrns,omitempty"`
	EstimatedBytes int64    `json:"estimated_bytes,omitempty"`
}

// QueryColumn describes one column of a query result.
type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// QueryResultChunk is one batch of query results. Columns is set on the
// first chunk only.
type QueryResultChunk struct {
	Columns []QueryColumn `json:"columns,omitempty"`
	Rows    [][]any       `json:"rows,omitempty"`
}

// Meta describes a plugin to the Marmot host: identity, display
// information, and the config spec used to render its settings form.
// Status is one of "stable", "beta", or "experimental". Features lists
// the asset kinds the plugin produces (e.g. "Assets", "Lineage", "Run
// History"). SupportsDataPreview and SupportsQuery are set by Serve when
// the plugin's Source implements DataFetcher or Querier; plugin authors
// never set them.
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
	SupportsQuery       bool          `json:"supports_query,omitempty"`
}
