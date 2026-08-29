// Package sql extracts column-level lineage from SQL queries using
// Bytebase's per-dialect QuerySpan parsers.
//
// To add a dialect, extend dialectToEngine and blank-import the matching
// parser package so it registers itself with the base dispatcher.
package sql

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/store/model"

	// Blank imports register per-dialect GetQuerySpan implementations with
	// the base dispatcher via package init(); pair each one with a
	// dialectToEngine entry.
	_ "github.com/bytebase/bytebase/backend/plugin/parser/pg"

	pluginsdk "github.com/marmotdata/plugin-sdk"
)

// defaultParseTimeout mirrors DataHub's 10-second cooperative timeout for
// sqlglot: long enough for real queries but short enough that a pathological
// input cannot stall ingest.
const defaultParseTimeout = 10 * time.Second

// ErrUnsupportedDialect is returned by Extract when the request's Dialect has
// no registered parser. Use Supported to check up front.
var ErrUnsupportedDialect = errors.New("unsupported sql dialect")

// ExtractRequest carries the SQL plus the parser context: DefaultDatabase and
// DefaultSchema seed name resolution for unqualified references and Sources
// supplies the schemas needed for SELECT * expansion and column
// disambiguation.
type ExtractRequest struct {
	SQL             string
	Dialect         string
	DefaultDatabase string
	DefaultSchema   string
	Sources         []ExtractSource
}

// ExtractSource is one table the parser may reference; MRN is opaque to the
// extractor and only used by the caller to attribute derived edges.
type ExtractSource struct {
	MRN      string
	Database string
	Schema   string
	Table    string
	Columns  map[string]string
}

// Extractor derives column-level lineage from a SQL query and returns edges
// keyed by source-table MRN so callers can demux them onto the correct
// LineageEdge. The zero value is ready to use and safe for concurrent use.
type Extractor struct {
	// Timeout bounds a single parse; zero means 10 seconds.
	Timeout time.Duration
}

// dialectToEngine maps ExtractRequest.Dialect strings to Bytebase's engine
// enum and should grow alongside the blank imports at the top.
var dialectToEngine = map[string]storepb.Engine{
	"postgres":   storepb.Engine_POSTGRES,
	"postgresql": storepb.Engine_POSTGRES,
	"pg":         storepb.Engine_POSTGRES,
}

// Supported reports whether Extract can parse the given dialect.
func Supported(dialect string) bool {
	_, ok := dialectToEngine[dialect]
	return ok
}

// Extract parses req.SQL and returns column edges keyed by source-table MRN.
// It returns ErrUnsupportedDialect when req.Dialect has no parser and a parse
// error when the SQL cannot be analyzed; callers typically treat any error as
// a cue to fall back to table-only lineage.
func (e *Extractor) Extract(ctx context.Context, req ExtractRequest) (map[string][]pluginsdk.ColumnEdge, error) {
	if req.SQL == "" {
		return nil, nil
	}
	engine, ok := dialectToEngine[req.Dialect]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedDialect, req.Dialect)
	}
	req.DefaultDatabase = cmp.Or(req.DefaultDatabase, "marmot")
	req.DefaultSchema = cmp.Or(req.DefaultSchema, "public")

	ctx, cancel := context.WithTimeout(ctx, cmp.Or(e.Timeout, defaultParseTimeout))
	defer cancel()

	meta, tableToMRN := buildDatabaseMetadata(req, engine)

	gCtx := base.GetQuerySpanContext{
		GetDatabaseMetadataFunc: func(_ context.Context, _, _ string) (string, *model.DatabaseMetadata, error) {
			return req.DefaultDatabase, meta, nil
		},
	}

	spans, err := safeGetQuerySpan(ctx, engine, gCtx, req.SQL, req.DefaultDatabase, req.DefaultSchema)
	if err != nil {
		return nil, fmt.Errorf("parsing %s query span: %w", req.Dialect, err)
	}

	out := map[string][]pluginsdk.ColumnEdge{}
	for _, span := range spans {
		if span == nil {
			continue
		}
		for mrn, edges := range demuxSpan(span, tableToMRN) {
			out[mrn] = append(out[mrn], edges...)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// safeGetQuerySpan invokes the dispatcher and turns panics into errors so a
// parser bug cannot crash the caller's discovery goroutine.
func safeGetQuerySpan(ctx context.Context, engine storepb.Engine, gCtx base.GetQuerySpanContext, sql, database, schema string) (spans []*base.QuerySpan, err error) {
	defer func() {
		if r := recover(); r != nil {
			spans = nil
			err = fmt.Errorf("parser panicked: %v", r)
		}
	}()
	return base.GetQuerySpan(ctx, gCtx, engine, []base.Statement{{Text: sql}}, database, schema, false)
}

type tableKey struct{ schema, table string }

// buildDatabaseMetadata translates ExtractRequest.Sources into the storepb
// shape Bytebase expects and returns a lookup used later to attribute derived
// edges back to a source MRN.
func buildDatabaseMetadata(req ExtractRequest, engine storepb.Engine) (*model.DatabaseMetadata, map[tableKey]string) {
	schemas := map[string]map[string]*storepb.TableMetadata{}
	tableToMRN := map[tableKey]string{}

	for _, src := range req.Sources {
		schemaName := cmp.Or(src.Schema, req.DefaultSchema)
		if schemas[schemaName] == nil {
			schemas[schemaName] = map[string]*storepb.TableMetadata{}
		}
		cols := make([]*storepb.ColumnMetadata, 0, len(src.Columns))
		for _, name := range slices.Sorted(maps.Keys(src.Columns)) {
			cols = append(cols, &storepb.ColumnMetadata{Name: name, Type: src.Columns[name]})
		}
		schemas[schemaName][src.Table] = &storepb.TableMetadata{Name: src.Table, Columns: cols}
		tableToMRN[tableKey{schema: schemaName, table: src.Table}] = src.MRN
	}
	if _, ok := schemas[req.DefaultSchema]; !ok {
		schemas[req.DefaultSchema] = map[string]*storepb.TableMetadata{}
	}

	schemaMetas := make([]*storepb.SchemaMetadata, 0, len(schemas))
	for _, name := range slices.Sorted(maps.Keys(schemas)) {
		tables := schemas[name]
		ts := make([]*storepb.TableMetadata, 0, len(tables))
		for _, tableName := range slices.Sorted(maps.Keys(tables)) {
			ts = append(ts, tables[tableName])
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

// demuxSpan groups the parser's per-column source references into ColumnEdges
// per source-table MRN; references to tables outside tableToMRN are dropped
// silently since they would produce dangling column edges with nowhere to
// hang.
func demuxSpan(span *base.QuerySpan, tableToMRN map[tableKey]string) map[string][]pluginsdk.ColumnEdge {
	type target struct {
		name    string
		isPlain bool
		sources map[string]map[string]struct{} // source MRN -> contributing columns
	}
	var targets []*target
	byName := map[string]*target{}

	for _, r := range span.Results {
		t := byName[r.Name]
		if t == nil {
			t = &target{name: r.Name, isPlain: r.IsPlainField, sources: map[string]map[string]struct{}{}}
			byName[r.Name] = t
			targets = append(targets, t)
		}
		for col := range r.SourceColumns {
			mrn, ok := tableToMRN[tableKey{schema: col.Schema, table: col.Table}]
			if !ok {
				// The parser normalises schema to "public" when the SQL omits
				// it, but plugin hints may not always match, so try the empty
				// schema as an alias before giving up.
				mrn, ok = tableToMRN[tableKey{table: col.Table}]
			}
			if !ok {
				continue
			}
			if t.sources[mrn] == nil {
				t.sources[mrn] = map[string]struct{}{}
			}
			t.sources[mrn][col.Column] = struct{}{}
		}
	}

	out := map[string][]pluginsdk.ColumnEdge{}
	for _, t := range targets {
		for _, mrn := range slices.Sorted(maps.Keys(t.sources)) {
			from := slices.Sorted(maps.Keys(t.sources[mrn]))
			transform := ""
			if !t.isPlain {
				transform = "expression"
			}
			// A single source column with the same name as the target is a
			// pass-through even when Bytebase flags it non-plain (GROUP BY
			// keys and other query-shape cases), and users read the
			// "expression" badge as "some maths happened here" so do not cry
			// wolf.
			if len(from) == 1 && from[0] == t.name {
				transform = ""
			}
			out[mrn] = append(out[mrn], pluginsdk.ColumnEdge{
				FromColumns: from,
				ToColumn:    t.name,
				Transform:   transform,
				Confidence:  1.0,
			})
		}
	}
	return out
}
