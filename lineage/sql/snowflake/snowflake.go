// Package snowflake registers the Snowflake dialect with lineage/sql. Import
// it for its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/snowflake"
//
// Dialect: "snowflake". Sources set Database and Schema as Snowflake does.
package snowflake

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/snowflake"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

func init() {
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_SNOWFLAKE, DefaultSchema: "PUBLIC"}, "snowflake")
}
