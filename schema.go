package pluginsdk

import (
	"encoding/json"
	"fmt"
)

// schemaColumnsKey is the Asset.Schema entry that holds the column list.
const schemaColumnsKey = "columns"

// Column is the canonical shape for one column of a table-shaped asset's
// schema. Attach a []Column to an asset with Asset.SetColumns; Marmot renders
// the list as the "Formatted" view on the asset's Schema tab.
//
// Marmot recognizes a schema as a column list when the first element carries a
// string column_name and a string data_type, so both are always emitted. Every
// other field is optional and omitted from the JSON when unset, except Nullable
// (see below).
type Column struct {
	// Name is the column name. Required.
	Name string `json:"column_name"`
	// DataType is the column's type, shown as the type badge. Required.
	DataType string `json:"data_type"`
	// Nullable drives the Required/Optional badge: a non-nullable column
	// (false) renders as Required. It is always emitted, so every column gets a
	// badge. Build the schema map by hand if you need to leave nullability
	// unknown.
	Nullable bool `json:"is_nullable"`
	// PrimaryKey adds a "Primary Key" annotation when true.
	PrimaryKey bool `json:"is_primary_key,omitempty"`
	// SortingKey adds a "Sorting Key" annotation when true.
	SortingKey bool `json:"is_sorting_key,omitempty"`
	// Comment is the column description.
	Comment string `json:"comment,omitempty"`
	// Default is the column's default value or expression, shown alongside it.
	Default any `json:"default_expression,omitempty"`
}

// SetColumns attaches cols to the asset as its schema column list. It marshals
// them to the JSON shape Marmot's schema view expects and stores the result
// under the "columns" key of Asset.Schema, initializing Schema when nil and
// leaving any other schema entries untouched.
func (a *Asset) SetColumns(cols []Column) error {
	data, err := json.Marshal(cols)
	if err != nil {
		return fmt.Errorf("marshaling columns: %w", err)
	}
	if a.Schema == nil {
		a.Schema = make(map[string]string)
	}
	a.Schema[schemaColumnsKey] = string(data)
	return nil
}
