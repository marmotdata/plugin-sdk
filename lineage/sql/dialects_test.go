package sql_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/lineage/sql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/bigquery"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/mysql"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/oracle"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/redshift"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/snowflake"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/trino"
	_ "github.com/marmotdata/plugin-sdk/lineage/sql/tsql"
)

// Every dialect gets the same view: a join of users and orders that copies
// two columns and computes full_name, with each engine's own name
// qualification and string concatenation.
func TestExtractDialects(t *testing.T) {
	sources := func(database, schema string, upper bool) []sql.ExtractSource {
		name := func(s string) string {
			if upper {
				return strings.ToUpper(s)
			}
			return s
		}
		return []sql.ExtractSource{
			{MRN: "mrn://users", Database: database, Schema: schema, Table: name("users"), Columns: map[string]string{name("id"): "int", name("first_name"): "text", name("last_name"): "text"}},
			{MRN: "mrn://orders", Database: database, Schema: schema, Table: name("orders"), Columns: map[string]string{name("id"): "int", name("user_id"): "int", name("total"): "numeric"}},
		}
	}

	tests := []struct {
		dialect   string
		sql       string
		database  string
		schema    string
		defSchema string
		// upper gives the sources upper-case names, as Oracle's and
		// Snowflake's catalogs report unquoted identifiers.
		upper bool
	}{
		{
			dialect:  "mysql",
			sql:      "SELECT u.id, CONCAT(u.first_name, ' ', u.last_name) AS full_name, o.total FROM shop.users u JOIN orders o ON o.user_id = u.id",
			database: "shop",
		},
		{
			dialect:  "mariadb",
			sql:      "SELECT u.id, CONCAT(u.first_name, ' ', u.last_name) AS full_name, o.total FROM `shop`.`users` u JOIN `orders` o ON o.user_id = u.id",
			database: "shop",
		},
		{
			dialect:  "mssql",
			sql:      "SELECT u.id, u.first_name + ' ' + u.last_name AS full_name, o.total FROM dbo.users u JOIN sales.dbo.orders o ON o.user_id = u.id",
			database: "sales",
			schema:   "dbo",
		},
		{
			dialect:  "oracle",
			sql:      "SELECT u.id, u.first_name || ' ' || u.last_name AS full_name, o.total FROM shop.users u JOIN shop.orders o ON o.user_id = u.id",
			database: "SHOP",
			upper:    true,
		},
		{
			dialect:   "trino",
			sql:       "SELECT u.id, concat(u.first_name, ' ', u.last_name) AS full_name, o.total FROM hive.shop.users u JOIN shop.orders o ON o.user_id = u.id",
			database:  "hive",
			schema:    "shop",
			defSchema: "shop",
		},
		{
			dialect:   "bigquery",
			sql:       "SELECT u.id, CONCAT(u.first_name, ' ', u.last_name) AS full_name, o.total FROM `my-project.shop.users` u JOIN `my-project`.`shop`.`orders` o ON o.user_id = u.id",
			database:  "my-project",
			schema:    "shop",
			defSchema: "shop",
		},
		{
			dialect:  "snowflake",
			sql:      "SELECT u.id, u.first_name || ' ' || u.last_name AS full_name, o.total FROM shop_db.public.users u JOIN orders o ON o.user_id = u.id",
			database: "SHOP_DB",
			schema:   "PUBLIC",
			upper:    true,
		},
		{
			dialect:  "redshift",
			sql:      "SELECT u.id, u.first_name || ' ' || u.last_name AS full_name, o.total FROM public.users u JOIN orders o ON o.user_id = u.id",
			database: "dev",
			schema:   "public",
		},
		{
			dialect:  "cockroachdb",
			sql:      "SELECT u.id, u.first_name || ' ' || u.last_name AS full_name, o.total FROM public.users u JOIN orders o ON o.user_id = u.id",
			database: "defaultdb",
			schema:   "public",
		},
	}

	for _, tt := range tests {
		t.Run(tt.dialect, func(t *testing.T) {
			require.True(t, sql.Supported(tt.dialect))
			var e sql.Extractor
			got, err := e.Extract(context.Background(), sql.ExtractRequest{
				SQL:             tt.sql,
				Dialect:         tt.dialect,
				DefaultDatabase: tt.database,
				DefaultSchema:   tt.defSchema,
				Sources:         sources(tt.database, tt.schema, tt.upper),
			})
			require.NoError(t, err)

			assert.Equal(t, []pluginsdk.ColumnEdge{
				{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1},
				{FromColumns: []string{"id"}, ToColumn: "id", Confidence: 1},
			}, normalise(got["mrn://users"]), "users edges")
			assert.Equal(t, []pluginsdk.ColumnEdge{
				{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1},
			}, normalise(got["mrn://orders"]), "orders edges")
		})
	}
}

// normalise lower-cases names (Oracle and Snowflake report upper case) and
// sorts edges so dialects can share expectations.
func normalise(edges []pluginsdk.ColumnEdge) []pluginsdk.ColumnEdge {
	out := make([]pluginsdk.ColumnEdge, 0, len(edges))
	for _, e := range edges {
		from := make([]string, len(e.FromColumns))
		for i, c := range e.FromColumns {
			from[i] = strings.ToLower(c)
		}
		slices.Sort(from)
		e.FromColumns = from
		e.ToColumn = strings.ToLower(e.ToColumn)
		out = append(out, e)
	}
	slices.SortFunc(out, func(a, b pluginsdk.ColumnEdge) int { return strings.Compare(a.ToColumn, b.ToColumn) })
	return out
}

func TestExtractUnregisteredDialect(t *testing.T) {
	var e sql.Extractor
	_, err := e.Extract(context.Background(), sql.ExtractRequest{SQL: "SELECT 1", Dialect: "clickhouse"})
	require.ErrorIs(t, err, sql.ErrUnsupportedDialect)
	assert.False(t, sql.Supported("clickhouse"))
}

// Redshift's parser drops a query whose column types it cannot check; the
// PostgreSQL fallback still recovers the lineage.
func TestExtractRedshiftUntypedColumns(t *testing.T) {
	var e sql.Extractor
	got, err := e.Extract(context.Background(), sql.ExtractRequest{
		SQL:     "SELECT id, amount * 2 AS double_amount FROM public.orders",
		Dialect: "redshift",
		Sources: []sql.ExtractSource{{MRN: "mrn://orders", Schema: "public", Table: "orders", Columns: map[string]string{"id": "", "amount": ""}}},
	})
	require.NoError(t, err)
	assert.Equal(t, []pluginsdk.ColumnEdge{
		{FromColumns: []string{"amount"}, ToColumn: "double_amount", Transform: "expression", Confidence: 1},
		{FromColumns: []string{"id"}, ToColumn: "id", Confidence: 1},
	}, normalise(got["mrn://orders"]))
}

// Bytebase skips Oracle's HR and SCOTT sample schemas; a user schema of that
// name still gets lineage, qualified or as the default.
func TestExtractOracleReservedSchema(t *testing.T) {
	sources := []sql.ExtractSource{{MRN: "mrn://employees", Database: "HR", Table: "EMPLOYEES", Columns: map[string]string{"ID": "NUMBER", "SALARY": "NUMBER"}}}
	for _, query := range []string{
		"SELECT e.id, e.salary * 12 AS annual FROM hr.employees e",
		`SELECT e.id, e.salary * 12 AS annual FROM "HR"."EMPLOYEES" e`,
		"SELECT id, salary * 12 AS annual FROM employees",
	} {
		var e sql.Extractor
		got, err := e.Extract(context.Background(), sql.ExtractRequest{SQL: query, Dialect: "oracle", DefaultDatabase: "HR", Sources: sources})
		require.NoError(t, err, query)
		assert.Equal(t, []pluginsdk.ColumnEdge{
			{FromColumns: []string{"salary"}, ToColumn: "annual", Transform: "expression", Confidence: 1},
			{FromColumns: []string{"id"}, ToColumn: "id", Confidence: 1},
		}, normalise(got["mrn://employees"]), query)
	}
}
