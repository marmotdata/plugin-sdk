package pluginsdk

import (
	"encoding/json"
	"fmt"
)

// schemaColumnsKey is the Asset.Schema entry that holds the column list.
const schemaColumnsKey = "columns"

// Column is the canonical shape for a table column, filling one in renders
// correctly in Marmot's schema view with no guesswork about key names. It is
// not required: embed it to add source-specific fields, or pass a fully custom
// type to SetColumns.
type Column struct {
	// Name is the column name. Required.
	Name string `json:"column_name"`
	// DataType is shown as the type badge. Required.
	DataType string `json:"data_type"`
	// Nullable drives the Required/Optional badge. Always emitted, so false renders as Required.
	Nullable bool `json:"is_nullable"`
	// PrimaryKey adds a Primary Key annotation.
	PrimaryKey bool `json:"is_primary_key,omitempty"`
	// SortingKey adds a Sorting Key annotation.
	SortingKey bool `json:"is_sorting_key,omitempty"`
	// ForeignKey marks a foreign key. Recorded for column-level lineage, not rendered yet.
	ForeignKey bool `json:"is_foreign_key,omitempty"`
	// PII marks personally identifiable data. Recorded for column-level lineage, not rendered yet.
	PII bool `json:"is_pii,omitempty"`
	// Description is the column description.
	Description string `json:"description,omitempty"`
	// Default is the column's default value or expression.
	Default any `json:"default_expression,omitempty"`
}

// SetColumns sets the asset's column list to cols, storing them as JSON under
// the "columns" key of Asset.Schema and initializing Schema when nil. It
// replaces any existing list, build the whole slice (with Go's append, or a
// []any to mix shapes) and set it once.
//
// cols may be []Column, a type embedding Column with extra source-specific
// fields, or a []any mixing shapes. Each element just needs to marshal to an
// object with a string column_name and data_type.
func SetColumns[T any](a *Asset, cols []T) error {
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
