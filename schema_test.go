package pluginsdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetColumnsStoresUnderColumnsKey(t *testing.T) {
	var a Asset
	err := SetColumns(&a, []Column{{Name: "id", DataType: "INTEGER"}})
	require.NoError(t, err)

	raw, ok := a.Schema[schemaColumnsKey]
	require.True(t, ok, "columns key should be set")

	var cols []Column
	require.NoError(t, json.Unmarshal([]byte(raw), &cols))
	require.Len(t, cols, 1)
	assert.Equal(t, "id", cols[0].Name)
	assert.Equal(t, "INTEGER", cols[0].DataType)
}

func TestSetColumnsInitializesNilSchema(t *testing.T) {
	a := Asset{}
	require.Nil(t, a.Schema)

	require.NoError(t, SetColumns(&a, []Column{{Name: "id", DataType: "INTEGER"}}))
	assert.NotNil(t, a.Schema)
}

func TestSetColumnsPreservesOtherSchemaEntries(t *testing.T) {
	a := Asset{Schema: map[string]string{"raw": "keep-me"}}

	require.NoError(t, SetColumns(&a, []Column{{Name: "id", DataType: "INTEGER"}}))
	assert.Equal(t, "keep-me", a.Schema["raw"])
}

func TestSetColumnsEmitsColumnNameAndDataTypeForDetection(t *testing.T) {
	// Marmot detects the SQL column format by the presence of string
	// column_name and data_type on the first element, so both must always
	// serialize even when empty.
	var a Asset
	require.NoError(t, SetColumns(&a, []Column{{}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1)

	_, hasName := cols[0]["column_name"]
	_, hasType := cols[0]["data_type"]
	assert.True(t, hasName, "column_name must always be present")
	assert.True(t, hasType, "data_type must always be present")
}

func TestSetColumnsAlwaysEmitsIsNullable(t *testing.T) {
	// is_nullable has no omitempty: a non-nullable column (false) must still
	// serialize so the Formatted view can render the Required badge.
	var a Asset
	require.NoError(t, SetColumns(&a, []Column{{Name: "id", DataType: "INTEGER", Nullable: false}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1)

	val, ok := cols[0]["is_nullable"]
	require.True(t, ok, "is_nullable must be present even when false")
	assert.Equal(t, false, val)
}

func TestSetColumnsOmitsUnsetOptionalFields(t *testing.T) {
	var a Asset
	require.NoError(t, SetColumns(&a, []Column{{Name: "id", DataType: "INTEGER"}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1)

	assert.NotContains(t, cols[0], "is_primary_key")
	assert.NotContains(t, cols[0], "is_sorting_key")
	assert.NotContains(t, cols[0], "comment")
	assert.NotContains(t, cols[0], "default_expression")
}

func TestSetColumnsUsesCanonicalKeyNames(t *testing.T) {
	var a Asset
	require.NoError(t, SetColumns(&a, []Column{{
		Name:       "id",
		DataType:   "INTEGER",
		Nullable:   false,
		PrimaryKey: true,
		SortingKey: true,
		Comment:    "Surrogate key",
		Default:    "nextval('seq')",
	}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1)

	assert.Equal(t, "id", cols[0]["column_name"])
	assert.Equal(t, "INTEGER", cols[0]["data_type"])
	assert.Equal(t, true, cols[0]["is_primary_key"])
	assert.Equal(t, true, cols[0]["is_sorting_key"])
	assert.Equal(t, "Surrogate key", cols[0]["comment"])
	assert.Equal(t, "nextval('seq')", cols[0]["default_expression"])
}

func TestSetColumnsFlattensEmbeddedExtraFields(t *testing.T) {
	// Embedding Column and adding fields must produce one flat JSON object:
	// canonical keys the view reads, plus the source-specific extras.
	type richColumn struct {
		Column
		OrdinalPosition int  `json:"ordinal_position"`
		AutoIncrement   bool `json:"is_auto_increment"`
	}

	var a Asset
	require.NoError(t, SetColumns(&a, []richColumn{{
		Column:          Column{Name: "id", DataType: "INTEGER", Nullable: false, PrimaryKey: true},
		OrdinalPosition: 1,
		AutoIncrement:   true,
	}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1)

	// canonical fields present, flattened (not nested under a "Column" key)
	assert.Equal(t, "id", cols[0]["column_name"])
	assert.Equal(t, true, cols[0]["is_primary_key"])
	assert.NotContains(t, cols[0], "Column")
	// extras preserved alongside; JSON numbers decode to float64
	assert.Equal(t, float64(1), cols[0]["ordinal_position"])
	assert.Equal(t, true, cols[0]["is_auto_increment"])
}

func TestSetColumnsAcceptsFullyCustomColumnType(t *testing.T) {
	// A source that does not fit Column (string is_nullable) can still use
	// SetColumns; it marshals what it is given, unchanged.
	type trinoColumn struct {
		ColumnName string `json:"column_name"`
		DataType   string `json:"data_type"`
		IsNullable string `json:"is_nullable"`
	}

	var a Asset
	require.NoError(t, SetColumns(&a, []trinoColumn{{ColumnName: "id", DataType: "bigint", IsNullable: "NO"}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1)
	assert.Equal(t, "NO", cols[0]["is_nullable"], "string is_nullable preserved unchanged")
}

func TestSetColumnsOverwritesOnRepeatCall(t *testing.T) {
	// SetColumns replaces the "columns" section, it does not append: the last
	// call wins, even when the two calls use different column types.
	type trinoColumn struct {
		ColumnName string `json:"column_name"`
		DataType   string `json:"data_type"`
		IsNullable string `json:"is_nullable"`
	}

	var a Asset
	require.NoError(t, SetColumns(&a, []Column{{Name: "id", DataType: "bigint"}}))
	require.NoError(t, SetColumns(&a, []trinoColumn{{ColumnName: "payload", DataType: "json", IsNullable: "YES"}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1, "repeat call replaces the previous columns, it does not append")
	assert.Equal(t, "payload", cols[0]["column_name"])
	assert.Equal(t, "YES", cols[0]["is_nullable"])
}

func TestSetColumnsMixesCustomAndCanonicalInOneCall(t *testing.T) {
	// To combine column types in a single "columns" section, pass []any: each
	// element marshals by its own type, so canonical and custom shapes coexist.
	type trinoColumn struct {
		ColumnName string `json:"column_name"`
		DataType   string `json:"data_type"`
		IsNullable string `json:"is_nullable"`
	}

	var a Asset
	require.NoError(t, SetColumns(&a, []any{
		Column{Name: "id", DataType: "bigint", Nullable: false, PrimaryKey: true},
		trinoColumn{ColumnName: "payload", DataType: "json", IsNullable: "YES"},
	}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 2)

	// canonical element keeps its bool is_nullable and primary-key flag
	assert.Equal(t, "id", cols[0]["column_name"])
	assert.Equal(t, false, cols[0]["is_nullable"])
	assert.Equal(t, true, cols[0]["is_primary_key"])

	// custom element keeps its string is_nullable untouched
	assert.Equal(t, "payload", cols[1]["column_name"])
	assert.Equal(t, "YES", cols[1]["is_nullable"])
}
