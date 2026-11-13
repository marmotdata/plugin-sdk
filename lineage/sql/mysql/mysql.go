// Package mysql registers the MySQL dialect with lineage/sql. Import it for
// its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/mysql"
//
// Dialects: "mysql" and "mariadb". MySQL has no schema level, so sources set
// Database (the MySQL schema) and leave Schema empty. Doris and StarRocks
// speak MySQL's SQL and can use "mysql" for their view definitions.
package mysql

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/mysql"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

func init() {
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_MYSQL}, "mysql")
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_MARIADB}, "mariadb")
}
