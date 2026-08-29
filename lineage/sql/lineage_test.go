package sql_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/lineage/sql"
)

var testSources = []sql.ExtractSource{
	{
		MRN:    "mrn://users",
		Schema: "public",
		Table:  "users",
		Columns: map[string]string{
			"id":         "integer",
			"first_name": "text",
			"last_name":  "text",
		},
	},
	{
		MRN:    "mrn://orders",
		Schema: "public",
		Table:  "orders",
		Columns: map[string]string{
			"id":      "integer",
			"user_id": "integer",
			"total":   "numeric",
		},
	},
}

func TestExtract(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want map[string][]pluginsdk.ColumnEdge
	}{
		{
			name: "pass-through select",
			sql:  "SELECT id, first_name FROM users",
			want: map[string][]pluginsdk.ColumnEdge{
				"mrn://users": {
					{FromColumns: []string{"id"}, ToColumn: "id", Confidence: 1.0},
					{FromColumns: []string{"first_name"}, ToColumn: "first_name", Confidence: 1.0},
				},
			},
		},
		{
			name: "expression column",
			sql:  "SELECT upper(first_name) AS uname FROM users",
			want: map[string][]pluginsdk.ColumnEdge{
				"mrn://users": {
					{FromColumns: []string{"first_name"}, ToColumn: "uname", Transform: "expression", Confidence: 1.0},
				},
			},
		},
		{
			name: "multi-column expression",
			sql:  "SELECT first_name || ' ' || last_name AS full_name FROM users",
			want: map[string][]pluginsdk.ColumnEdge{
				"mrn://users": {
					{FromColumns: []string{"first_name", "last_name"}, ToColumn: "full_name", Transform: "expression", Confidence: 1.0},
				},
			},
		},
		{
			name: "join across two tables",
			sql:  "SELECT u.first_name, o.total FROM users u JOIN orders o ON o.user_id = u.id",
			want: map[string][]pluginsdk.ColumnEdge{
				"mrn://users": {
					{FromColumns: []string{"first_name"}, ToColumn: "first_name", Confidence: 1.0},
				},
				"mrn://orders": {
					{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1.0},
				},
			},
		},
		{
			name: "select star expands to all columns",
			sql:  "SELECT * FROM orders",
			want: map[string][]pluginsdk.ColumnEdge{
				"mrn://orders": {
					{FromColumns: []string{"id"}, ToColumn: "id", Confidence: 1.0},
					{FromColumns: []string{"total"}, ToColumn: "total", Confidence: 1.0},
					{FromColumns: []string{"user_id"}, ToColumn: "user_id", Confidence: 1.0},
				},
			},
		},
		{
			name: "same-name column is a pass-through even when grouped",
			sql:  "SELECT user_id FROM orders GROUP BY user_id",
			want: map[string][]pluginsdk.ColumnEdge{
				"mrn://orders": {
					{FromColumns: []string{"user_id"}, ToColumn: "user_id", Confidence: 1.0},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e sql.Extractor
			got, err := e.Extract(t.Context(), sql.ExtractRequest{
				SQL:     tt.sql,
				Dialect: "postgres",
				Sources: testSources,
			})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractEmptySQL(t *testing.T) {
	var e sql.Extractor
	got, err := e.Extract(t.Context(), sql.ExtractRequest{Dialect: "postgres"})
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestExtractUnsupportedDialect(t *testing.T) {
	var e sql.Extractor
	_, err := e.Extract(t.Context(), sql.ExtractRequest{
		SQL:     "SELECT 1",
		Dialect: "cobol",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, sql.ErrUnsupportedDialect))
}

func TestExtractParseError(t *testing.T) {
	var e sql.Extractor
	_, err := e.Extract(t.Context(), sql.ExtractRequest{
		SQL:     "SELECT FROM WHERE (",
		Dialect: "postgres",
	})
	assert.Error(t, err)
}

func TestSupported(t *testing.T) {
	assert.True(t, sql.Supported("postgres"))
	assert.True(t, sql.Supported("postgresql"))
	assert.True(t, sql.Supported("pg"))
	assert.False(t, sql.Supported("mysql"))
}
