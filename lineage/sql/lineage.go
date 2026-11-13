// Package sql extracts column-level lineage from SQL queries using
// Bytebase's per-dialect QuerySpan parsers.
//
// No dialect is built in: import the subpackage for each dialect a plugin
// parses, so it only links the parsers it uses. For example:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/mysql"
//
// To add a dialect, add a subpackage that registers it with
// internal/dialect and blank-imports the matching Bytebase parser.
package sql

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/parser/base"
	"github.com/bytebase/bytebase/backend/store/model"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

// defaultParseTimeout mirrors DataHub's 10-second cooperative timeout for
// sqlglot: long enough for real queries but short enough that a pathological
// input cannot stall ingest.
const defaultParseTimeout = 10 * time.Second

// ErrUnsupportedDialect is returned by Extract when the request's Dialect has
// no registered parser, usually because its subpackage is not imported. Use
// Supported to check up front.
var ErrUnsupportedDialect = errors.New("unsupported sql dialect")

// ExtractRequest carries the SQL plus the parser context: DefaultDatabase and
// DefaultSchema seed name resolution for unqualified references and Sources
// supplies the schemas needed for SELECT * expansion and column
// disambiguation. An empty DefaultSchema takes the dialect's default (public
// for PostgreSQL, dbo for SQL Server, none for MySQL).
type ExtractRequest struct {
	SQL             string
	Dialect         string
	DefaultDatabase string
	DefaultSchema   string
	Sources         []ExtractSource
}

// ExtractSource is one table the parser may reference; MRN is opaque to the
// extractor and only used by the caller to attribute derived edges. Database
// is the engine's top level (database, catalog or project) and defaults to
// the request's DefaultDatabase; Schema is the level below it (schema or
// dataset) and is empty for engines without one.
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

// Supported reports whether Extract can parse the given dialect.
func Supported(dialectName string) bool {
	_, ok := dialect.Lookup(dialectName)
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
	d, ok := dialect.Lookup(req.Dialect)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedDialect, req.Dialect)
	}
	engine := d.Engine
	if d.SchemaIsDatabase {
		req = schemaAsDatabase(req)
	}
	if d.RewriteSQL != nil {
		req.SQL = d.RewriteSQL(req.SQL)
	}
	req.DefaultDatabase = cmp.Or(req.DefaultDatabase, "marmot")
	req.DefaultSchema = cmp.Or(req.DefaultSchema, d.DefaultSchema)
	req = renameReserved(req, d.ReservedDatabases)

	out, err := e.extract(ctx, req, engine)
	if err == nil && out == nil && d.Fallback != "" {
		if fb, ok := dialect.Lookup(d.Fallback); ok {
			return e.extract(ctx, req, fb.Engine)
		}
	}
	return out, err
}

func (e *Extractor) extract(ctx context.Context, req ExtractRequest, engine storepb.Engine) (map[string][]pluginsdk.ColumnEdge, error) {

	ctx, cancel := context.WithTimeout(ctx, cmp.Or(e.Timeout, defaultParseTimeout))
	defer cancel()

	metas, idx := buildDatabaseMetadata(req, engine)

	gCtx := base.GetQuerySpanContext{
		GetDatabaseMetadataFunc: func(_ context.Context, _, database string) (string, *model.DatabaseMetadata, error) {
			if m, ok := metas[fold(database)]; ok {
				return m.name, m.meta, nil
			}
			// An unknown database is one the plugin did not supply: hand
			// back an empty one so its references resolve to nothing
			// rather than failing the whole query.
			return database, newDatabaseMetadata(database, req.DefaultSchema, nil, engine), nil
		},
		ListDatabaseNamesFunc: func(context.Context, string) ([]string, error) {
			names := make([]string, 0, len(metas))
			for _, m := range metas {
				names = append(names, m.name)
			}
			slices.Sort(names)
			return names, nil
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
		for mrn, edges := range demuxSpan(span, idx) {
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

// reservedPrefix renames databases the parser would ignore; see
// Dialect.ReservedDatabases.
const reservedPrefix = "MARMOT_"

// renameReserved presents sources in a reserved database under a placeholder
// name and rewrites the SQL's qualified references to match. It leaves a
// query alone when it uses the reserved name as a table alias, since the
// rewrite could not tell the two apart.
func renameReserved(req ExtractRequest, reserved []string) ExtractRequest {
	for _, name := range reserved {
		used := strings.EqualFold(req.DefaultDatabase, name)
		for _, src := range req.Sources {
			used = used || strings.EqualFold(src.Database, name)
		}
		if !used || aliasPattern(name).MatchString(req.SQL) {
			continue
		}
		placeholder := reservedPrefix + strings.ToUpper(name)
		req.SQL = qualifierPattern(name).ReplaceAllString(req.SQL, "${1}${2}"+placeholder+"${3}${4}")
		if strings.EqualFold(req.DefaultDatabase, name) {
			req.DefaultDatabase = placeholder
		}
		sources := make([]ExtractSource, len(req.Sources))
		for i, src := range req.Sources {
			if strings.EqualFold(src.Database, name) {
				src.Database = placeholder
			}
			sources[i] = src
		}
		req.Sources = sources
	}
	return req
}

// qualifierPattern matches name used as a qualifier: HR.EMPLOYEES or
// "HR"."EMPLOYEES", but not X.HR.
func qualifierPattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|[^\w$#".])(")?` + regexp.QuoteMeta(name) + `(")?(\s*\.)`)
}

// aliasPattern matches name declared as a table alias: FROM t hr, JOIN t AS hr.
func aliasPattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+[\w$#".]+\s+(?:AS\s+)?"?` + regexp.QuoteMeta(name) + `"?(?:\s|$|,|\))`)
}

// schemaAsDatabase moves each source's Schema up to Database, for engines
// whose parser has a single level above tables (see Dialect.SchemaIsDatabase).
func schemaAsDatabase(req ExtractRequest) ExtractRequest {
	req.DefaultDatabase = cmp.Or(req.DefaultSchema, req.DefaultDatabase)
	req.DefaultSchema = ""
	sources := make([]ExtractSource, len(req.Sources))
	for i, src := range req.Sources {
		src.Database = cmp.Or(src.Schema, src.Database)
		src.Schema = ""
		sources[i] = src
	}
	req.Sources = sources
	return req
}

// fold normalises an identifier for matching: engines disagree on case
// (Oracle reports upper case, PostgreSQL lower) and the plugin's names may
// come from a different source than the parser's.
func fold(name string) string { return strings.ToLower(name) }

type tableKey struct{ database, schema, table string }

// mrnIndex finds the source MRN for a table the parser reported, trying the
// most specific name first: database.schema.table, then schema.table, then
// the table alone when only one source has that name.
type mrnIndex struct {
	full        map[tableKey]string
	schemaTable map[tableKey]string
	table       map[string][]string
}

func (x mrnIndex) lookup(database, schema, table string) (string, bool) {
	if mrn, ok := x.full[tableKey{fold(database), fold(schema), fold(table)}]; ok {
		return mrn, true
	}
	if mrn, ok := x.schemaTable[tableKey{schema: fold(schema), table: fold(table)}]; ok && schema != "" {
		return mrn, true
	}
	if mrns := x.table[fold(table)]; len(mrns) == 1 {
		return mrns[0], true
	}
	return "", false
}

type databaseMetadata struct {
	name string
	meta *model.DatabaseMetadata
}

// buildDatabaseMetadata translates ExtractRequest.Sources into the storepb
// shape Bytebase expects, one database at a time, and returns an index used
// later to attribute derived edges back to a source MRN.
func buildDatabaseMetadata(req ExtractRequest, engine storepb.Engine) (map[string]databaseMetadata, mrnIndex) {
	type database struct {
		name    string
		schemas map[string]map[string]*storepb.TableMetadata
	}
	databases := map[string]*database{}
	idx := mrnIndex{full: map[tableKey]string{}, schemaTable: map[tableKey]string{}, table: map[string][]string{}}

	for _, src := range req.Sources {
		dbName := cmp.Or(src.Database, req.DefaultDatabase)
		db := databases[fold(dbName)]
		if db == nil {
			db = &database{name: dbName, schemas: map[string]map[string]*storepb.TableMetadata{}}
			databases[fold(dbName)] = db
		}
		schemaName := cmp.Or(src.Schema, req.DefaultSchema)
		if db.schemas[schemaName] == nil {
			db.schemas[schemaName] = map[string]*storepb.TableMetadata{}
		}
		cols := make([]*storepb.ColumnMetadata, 0, len(src.Columns))
		for _, name := range slices.Sorted(maps.Keys(src.Columns)) {
			cols = append(cols, &storepb.ColumnMetadata{Name: name, Type: src.Columns[name]})
		}
		db.schemas[schemaName][src.Table] = &storepb.TableMetadata{Name: src.Table, Columns: cols}

		idx.full[tableKey{fold(dbName), fold(schemaName), fold(src.Table)}] = src.MRN
		idx.schemaTable[tableKey{schema: fold(schemaName), table: fold(src.Table)}] = src.MRN
		if !slices.Contains(idx.table[fold(src.Table)], src.MRN) {
			idx.table[fold(src.Table)] = append(idx.table[fold(src.Table)], src.MRN)
		}
	}
	if databases[fold(req.DefaultDatabase)] == nil {
		databases[fold(req.DefaultDatabase)] = &database{name: req.DefaultDatabase, schemas: map[string]map[string]*storepb.TableMetadata{}}
	}

	out := make(map[string]databaseMetadata, len(databases))
	for key, db := range databases {
		out[key] = databaseMetadata{name: db.name, meta: newDatabaseMetadata(db.name, req.DefaultSchema, db.schemas, engine)}
	}
	return out, idx
}

func newDatabaseMetadata(name, defaultSchema string, schemas map[string]map[string]*storepb.TableMetadata, engine storepb.Engine) *model.DatabaseMetadata {
	if schemas == nil {
		schemas = map[string]map[string]*storepb.TableMetadata{}
	}
	if _, ok := schemas[defaultSchema]; !ok {
		schemas[defaultSchema] = map[string]*storepb.TableMetadata{}
	}
	schemaMetas := make([]*storepb.SchemaMetadata, 0, len(schemas))
	for _, schemaName := range slices.Sorted(maps.Keys(schemas)) {
		tables := schemas[schemaName]
		ts := make([]*storepb.TableMetadata, 0, len(tables))
		for _, tableName := range slices.Sorted(maps.Keys(tables)) {
			ts = append(ts, tables[tableName])
		}
		schemaMetas = append(schemaMetas, &storepb.SchemaMetadata{Name: schemaName, Tables: ts})
	}
	proto := &storepb.DatabaseSchemaMetadata{
		Name:       name,
		Schemas:    schemaMetas,
		SearchPath: defaultSchema,
	}
	return model.NewDatabaseMetadata(proto, nil, nil, engine, false)
}

// demuxSpan groups the parser's per-column source references into ColumnEdges
// per source-table MRN; references to tables outside the index are dropped
// silently since they would produce dangling column edges with nowhere to
// hang.
func demuxSpan(span *base.QuerySpan, idx mrnIndex) map[string][]pluginsdk.ColumnEdge {
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
			mrn, ok := idx.lookup(col.Database, col.Schema, col.Table)
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
			// wolf. Case is ignored because some parsers upper-case result
			// names but not source columns.
			if len(from) == 1 && strings.EqualFold(from[0], t.name) {
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
