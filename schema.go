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
	// DataType is the column's type, shown as the type badge. Required.
	DataType string `json:"data_type"`
	// Nullable drives the Required/Optional badge: a non-nullable column
	// (false) renders as Required. It is always emitted, so every column gets a
	// badge. A source with no notion of nullability should use its own column
	// type rather than Column, so it does not report a spurious Required badge.
	Nullable bool `json:"is_nullable"`
	// PrimaryKey adds a "Primary Key" annotation when true.
	PrimaryKey bool `json:"is_primary_key,omitempty"`
	// SortingKey adds a "Sorting Key" annotation when true.
	SortingKey bool `json:"is_sorting_key,omitempty"`
	// ForeignKey marks the column as a foreign key. Recorded ahead of
	// column-level lineage; the schema view does not render it yet.
	ForeignKey bool `json:"is_foreign_key,omitempty"`
	// PII marks the column as holding personally identifiable information.
	// Recorded ahead of column-level lineage and governance; the schema view
	// does not render it yet.
	PII bool `json:"is_pii,omitempty"`
	// Comment is the column description.
	Comment string `json:"comment,omitempty"`
	// Default is the column's default value or expression, shown alongside it.
	Default any `json:"default_expression,omitempty"`
}

// SetColumns attaches a column list to the asset's schema. It marshals cols to
// JSON and stores the result under the "columns" key of Asset.Schema,
// initializing Schema when nil and leaving any other schema sections untouched.
//
// Each call replaces the columns section, it does not append. Build the whole
// list and set it once; to combine column shapes, pass a single slice such as
// []any whose elements each marshal to the recognized object.
//
// cols may be a slice of any type that marshals to a JSON object with a string
// column_name and a string data_type, which is how Marmot recognizes the column
// format. Use []Column for the canonical fields:
//
//	err := pluginsdk.SetColumns(asset, []pluginsdk.Column{
//		{Name: "id", DataType: "INTEGER", Nullable: false, PrimaryKey: true},
//	})
//
// To carry source-specific fields, embed Column in your own type. Embedding
// flattens Column's fields into the same JSON object, so the view reads the
// canonical fields while the extras are preserved in the Raw view:
//
//	type clickhouseColumn struct {
//		pluginsdk.Column
//		Codec string `json:"codec,omitempty"`
//		TTL   string `json:"ttl,omitempty"`
//	}
//	err := pluginsdk.SetColumns(asset, []clickhouseColumn{ ... })
//
// A source whose columns do not map onto Column (no notion of nullability, or
// one that already has its own column type) can pass a fully custom slice;
// SetColumns only marshals it.
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
