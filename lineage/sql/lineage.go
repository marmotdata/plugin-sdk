// Package sqllineage extracts column-level lineage from SQL queries using Bytebase's per-dialect QuerySpan parsers; add a dialect by extending dialectToEngine and blank-importing the matching parser, and note that Extract swallows parse failures so callers fall back to table-only lineage.
package sqllineage

import (
	"context"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/store/model"

	// Blank imports register per-dialect GetQuerySpan implementations with the base dispatcher via package init(); pair each one with a dialectToEngine entry.
	_ "github.com/bytebase/bytebase/backend/plugin/parser/pg"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// defaultParseTimeout mirrors DataHub's 10-second cooperative timeout for sqlglot: long enough for real queries but short enough that a pathological input cannot stall ingest.
const defaultParseTimeout = 10 * time.Second

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

// Extractor derives column-level lineage from a SQL query and returns edges keyed by source-table MRN so callers can demux them onto the correct LineageEdge; safe for concurrent use.
type Extractor struct {
	timeout time.Duration
}

// Option configures an Extractor.
type Option func(*Extractor)

// WithTimeout overrides the default 10s parse timeout.
func WithTimeout(d time.Duration) Option {
	return func(e *Extractor) { e.timeout = d }
}

// New returns a ready-to-use extractor.
func New(opts ...Option) *Extractor {
	e := &Extractor{timeout: defaultParseTimeout}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// dialectToEngine maps ExtractRequest.Dialect strings to Bytebase's engine enum and should grow alongside the blank imports at the top.
var dialectToEngine = map[string]storepb.Engine{
	"postgres":   storepb.Engine_POSTGRES,
	"postgresql": storepb.Engine_POSTGRES,
	"pg":         storepb.Engine_POSTGRES,
}

// Extract runs the extractor and returns column edges keyed by source-table MRN; parse failures return (nil, nil) so callers can fall back to table-only lineage.
func (e *Extractor) Extract(ctx context.Context, req ExtractRequest) (map[string][]pluginsdk.ColumnEdge, error) {
	if req.SQL == "" {
		return nil, nil
	}
	engine, ok := dialectToEngine[req.Dialect]
	if !ok {
		log.Info().Str("dialect", req.Dialect).Msg("sqllineage: unsupported dialect, skipping")
		return nil, nil
	}
	if req.DefaultDatabase == "" {
		req.DefaultDatabase = "marmot"
	}
	if req.DefaultSchema == "" {
		req.DefaultSchema = "public"
	}

	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	meta, tableToMRN := buildDatabaseMetadata(req, engine)

	gCtx := base.GetQuerySpanContext{
		GetDatabaseMetadataFunc: func(_ context.Context, _, _ string) (string, *model.DatabaseMetadata, error) {
			return req.DefaultDatabase, meta, nil
		},
	}

	spans, err := safeGetQuerySpan(ctx, engine, gCtx, req.SQL, req.DefaultDatabase, req.DefaultSchema)
	if err != nil {
		log.Warn().Err(err).Str("dialect", req.Dialect).Msg("sqllineage: parse failed, falling back to table-only")
		return nil, nil
	}
	if len(spans) == 0 {
		return nil, nil
	}

	out := map[string][]pluginsdk.ColumnEdge{}
	for _, span := range spans {
		if span == nil || len(span.Results) == 0 {
			continue
		}
		for mrn, edges := range demuxSpan(span, tableToMRN) {
			out[mrn] = append(out[mrn], edges...)
		}
	}
	return out, nil
}

// safeGetQuerySpan invokes the dispatcher and turns panics into errors so a parser bug cannot crash the caller's discovery goroutine.
func safeGetQuerySpan(ctx context.Context, engine storepb.Engine, gCtx base.GetQuerySpanContext, sql, database, schema string) (spans []*base.QuerySpan, err error) {
	defer func() {
		if r := recover(); r != nil {
			spans = nil
			err = errFromPanic(r)
		}
	}()
	return base.GetQuerySpan(ctx, gCtx, engine, []base.Statement{{Text: sql}}, database, schema, false)
}

type tableKey struct{ schema, table string }

// buildDatabaseMetadata translates ExtractRequest.Sources into the storepb shape Bytebase expects and returns a lookup used later to attribute derived edges back to a source MRN.
func buildDatabaseMetadata(req ExtractRequest, engine storepb.Engine) (*model.DatabaseMetadata, map[tableKey]string) {
	schemas := map[string]map[string]*storepb.TableMetadata{}
	tableToMRN := map[tableKey]string{}

	for _, src := range req.Sources {
		schemaName := src.Schema
		if schemaName == "" {
			schemaName = req.DefaultSchema
		}
		if schemas[schemaName] == nil {
			schemas[schemaName] = map[string]*storepb.TableMetadata{}
		}
		cols := make([]*storepb.ColumnMetadata, 0, len(src.Columns))
		for name, typ := range src.Columns {
			cols = append(cols, &storepb.ColumnMetadata{Name: name, Type: typ})
		}
		schemas[schemaName][src.Table] = &storepb.TableMetadata{Name: src.Table, Columns: cols}
		tableToMRN[tableKey{schema: schemaName, table: src.Table}] = src.MRN
	}
	if _, ok := schemas[req.DefaultSchema]; !ok {
		schemas[req.DefaultSchema] = map[string]*storepb.TableMetadata{}
	}

	schemaMetas := make([]*storepb.SchemaMetadata, 0, len(schemas))
	for name, tables := range schemas {
		ts := make([]*storepb.TableMetadata, 0, len(tables))
		for _, t := range tables {
			ts = append(ts, t)
		}
		schemaMetas = append(schemaMetas, &storepb.SchemaMetadata{Name: name, Tables: ts})
	}

	proto := &storepb.DatabaseSchemaMetadata{
		Name:       req.DefaultDatabase,
		Schemas:    schemaMetas,
		SearchPath: req.DefaultSchema,
	}
	return model.NewDatabaseMetadata(proto, nil, nil, engine, false), tableToMRN
}

// demuxSpan groups the parser's per-column source references into ColumnEdges per source-table MRN; references to tables outside tableToMRN are dropped silently since they would produce dangling column edges with nowhere to hang.
func demuxSpan(span *base.QuerySpan, tableToMRN map[tableKey]string) map[string][]pluginsdk.ColumnEdge {
	type key struct{ target, sourceMRN string }
	accum := map[key]map[string]struct{}{}
	targetIsPlain := map[string]bool{}
	targetOrder := []string{}
	seenTarget := map[string]bool{}

	for _, r := range span.Results {
		if !seenTarget[r.Name] {
			seenTarget[r.Name] = true
			targetOrder = append(targetOrder, r.Name)
			targetIsPlain[r.Name] = r.IsPlainField
		}
		for col := range r.SourceColumns {
			mrn, ok := tableToMRN[tableKey{schema: col.Schema, table: col.Table}]
			if !ok {
				// The parser normalises schema to "public" when the SQL omits it, but plugin hints may not always match, so try the empty schema as an alias before giving up.
				mrn, ok = tableToMRN[tableKey{schema: "", table: col.Table}]
			}
			if !ok {
				continue
			}
			k := key{target: r.Name, sourceMRN: mrn}
			if accum[k] == nil {
				accum[k] = map[string]struct{}{}
			}
			accum[k][col.Column] = struct{}{}
		}
	}

	out := map[string][]pluginsdk.ColumnEdge{}
	for _, target := range targetOrder {
		for k, cols := range accum {
			if k.target != target {
				continue
			}
			from := make([]string, 0, len(cols))
			for c := range cols {
				from = append(from, c)
			}
			sort.Strings(from)
			transform := ""
			if !targetIsPlain[target] {
				transform = "expression"
			}
			// A single source column with the same name as the target is a pass-through even when Bytebase flags it non-plain (GROUP BY keys and other query-shape cases), and users read the "expression" badge as "some maths happened here" so do not cry wolf.
			if len(from) == 1 && from[0] == target {
				transform = ""
			}
			out[k.sourceMRN] = append(out[k.sourceMRN], pluginsdk.ColumnEdge{
				FromColumns: from,
				ToColumn:    target,
				Transform:   transform,
				Confidence:  1.0,
			})
		}
	}
	return out
}

type parsePanicError struct{ val any }

func (e *parsePanicError) Error() string { return "sqllineage: parser panicked" }

func errFromPanic(r any) error { return &parsePanicError{val: r} }
