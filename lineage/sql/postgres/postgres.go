// Package postgres registers the PostgreSQL dialect with lineage/sql.
// Import it for its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/postgres"
//
// Dialects: "postgres", "postgresql" and "pg", plus "cockroachdb".
package postgres

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/pg"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

func init() {
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_POSTGRES, DefaultSchema: "public"}, "postgres", "postgresql", "pg")
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_COCKROACHDB, DefaultSchema: "public"}, "cockroachdb", "cockroach")
}
