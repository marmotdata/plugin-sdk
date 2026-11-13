// Package tsql registers the SQL Server (T-SQL) dialect with lineage/sql.
// Import it for its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/tsql"
//
// Dialects: "tsql", "mssql" and "sqlserver". Unqualified names resolve to
// the dbo schema unless the request says otherwise.
package tsql

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/tsql"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

func init() {
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_MSSQL, DefaultSchema: "dbo"}, "tsql", "mssql", "sqlserver")
}
