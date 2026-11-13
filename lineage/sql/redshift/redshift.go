// Package redshift registers the Amazon Redshift dialect with lineage/sql.
// Import it for its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/redshift"
//
// Dialect: "redshift". Redshift's parser type-checks the query and yields
// nothing when a column's type does not fit how it is used (a missing type in
// arithmetic, say), so an empty result is retried with the PostgreSQL parser.
package redshift

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/redshift"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/postgres"
)

func init() {
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_REDSHIFT, DefaultSchema: "public", Fallback: "postgres"}, "redshift")
}
