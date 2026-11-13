// Package oracle registers the Oracle (PL/SQL) dialect with lineage/sql.
// Import it for its side effect:
//
//	import _ "github.com/marmotdata/plugin-sdk/lineage/sql/oracle"
//
// Dialects: "oracle" and "plsql". Oracle's schema is its user, so sources set
// Database to the owning schema and leave Schema empty, with table and column
// names as the catalog reports them (upper case unless quoted). The parser
// ignores Oracle's own schemas; the HR and SCOTT sample schemas are handled
// under a placeholder name so user schemas called that still resolve.
package oracle

import (
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	_ "github.com/bytebase/bytebase/backend/plugin/parser/plsql"

	"github.com/marmotdata/plugin-sdk/lineage/sql/internal/dialect"
)

func init() {
	dialect.Register(dialect.Dialect{Engine: storepb.Engine_ORACLE, ReservedDatabases: []string{"HR", "SCOTT"}}, "oracle", "plsql")
}
