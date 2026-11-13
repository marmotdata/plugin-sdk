// Package bigquery registers the BigQuery (GoogleSQL) dialect with
// lineage/sql. Import it for its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/bigquery"
//
// Dialects: "bigquery" and "googlesql". Sources set Database to the project
// and Schema to the dataset, and the request's DefaultSchema to the default
// dataset. The parser knows datasets but not projects, so tables are matched
// by dataset and name: two projects with a dataset of the same name cannot be
// told apart in one request.
package bigquery

import (
	"regexp"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/bigquery"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

func init() {
	dialect.Register(dialect.Dialect{
		Engine:           storepb.Engine_BIGQUERY,
		SchemaIsDatabase: true,
		RewriteSQL:       dropProject,
	}, "bigquery", "googlesql")
}

var (
	// `project.dataset.table`, as BigQuery writes view definitions.
	quotedPath = regexp.MustCompile("`([^`.]+)\\.([^`.]+)\\.([^`.]+)`")
	// `project`.`dataset`.`table`
	quotedParts = regexp.MustCompile("`[^`.]+`\\.(`[^`.]+`\\.`[^`.]+`)")
)

// dropProject strips the project from three-part table names, leaving
// dataset.table for the parser to resolve.
func dropProject(sql string) string {
	sql = quotedPath.ReplaceAllString(sql, "`$2`.`$3`")
	return quotedParts.ReplaceAllString(sql, "$1")
}
