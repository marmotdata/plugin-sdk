package pluginsdk

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetColumnsStoresUnderColumnsKey(t *testing.T) {
	var a Asset
	err := a.SetColumns([]Column{{Name: "id", DataType: "INTEGER"}})
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

	require.NoError(t, a.SetColumns([]Column{{Name: "id", DataType: "INTEGER"}}))
	assert.NotNil(t, a.Schema)
}

func TestSetColumnsPreservesOtherSchemaEntries(t *testing.T) {
	a := Asset{Schema: map[string]string{"raw": "keep-me"}}

	require.NoError(t, a.SetColumns([]Column{{Name: "id", DataType: "INTEGER"}}))
	assert.Equal(t, "keep-me", a.Schema["raw"])
}

func TestSetColumnsEmitsColumnNameAndDataTypeForDetection(t *testing.T) {
	// Marmot detects the SQL column format by the presence of string
	// column_name and data_type on the first element, so both must always
	// serialize even when empty.
	var a Asset
	require.NoError(t, a.SetColumns([]Column{{}}))

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
	require.NoError(t, a.SetColumns([]Column{{Name: "id", DataType: "INTEGER", Nullable: false}}))

	var cols []map[string]any
	require.NoError(t, json.Unmarshal([]byte(a.Schema[schemaColumnsKey]), &cols))
	require.Len(t, cols, 1)

	val, ok := cols[0]["is_nullable"]
	require.True(t, ok, "is_nullable must be present even when false")
	assert.Equal(t, false, val)
}

func TestSetColumnsOmitsUnsetOptionalFields(t *testing.T) {
	var a Asset
	require.NoError(t, a.SetColumns([]Column{{Name: "id", DataType: "INTEGER"}}))

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
	require.NoError(t, a.SetColumns([]Column{{
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
