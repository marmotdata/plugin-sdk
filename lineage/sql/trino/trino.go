// Package trino registers the Trino dialect with lineage/sql. Import it for
// its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
//
// Dialects: "trino", "presto" and "athena". Sources set Database to the
// catalog and Schema to the schema.
package trino

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/trino"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

func init() {
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_TRINO}, "trino", "presto", "athena")
}
