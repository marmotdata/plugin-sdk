package pluginsdk

import "reflect"

// AssetSchema documents one shape of metadata a plugin attaches to the assets
// it discovers. The registry renders these as a plugin's "Assets Emitted".
type AssetSchema struct {
	StructName  string       `json:"struct_name"`
	DisplayName string       `json:"display_name"`
	Description string       `json:"description,omitempty"`
	Fields      []AssetField `json:"fields"`
}

// AssetField is one metadata key, taken from a `metadata:"..."` tag.
type AssetField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// AssetSchemaOf documents the metadata a plugin attaches to an asset.
// Fields come from the struct's `metadata:"..."` and `description:"..."` tags;
// untagged fields are left out. Assign the result to Meta.AssetSchemas:
//
//	AssetSchemas: []pluginsdk.AssetSchema{
//	    pluginsdk.AssetSchemaOf(SQLiteFields{}, "Table",
//	        "Tables and views discovered in a SQLite database file"),
//	    pluginsdk.AssetSchemaOf(SQLiteColumnFields{}, "Column",
//	        "Columns within a discovered table or view"),
//	},
//
// A value that is not a struct, or one with no tagged fields, yields a schema
// with no fields: documentation must not be able to stop a plugin from
// serving. Assert on Meta.AssetSchemas through plugintest to catch that.
func AssetSchemaOf(assetFields any, displayName, description string) AssetSchema {
	t := reflect.TypeOf(assetFields)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return AssetSchema{DisplayName: displayName, Description: description}
	}

	return AssetSchema{
		StructName:  t.Name(),
		DisplayName: displayName,
		Description: description,
		Fields:      assetFieldsOf(t),
	}
}

func assetFieldsOf(t reflect.Type) []AssetField {
	var fields []AssetField
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous || f.PkgPath != "" {
			continue
		}
		name := f.Tag.Get("metadata")
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, AssetField{
			Name:        name,
			Type:        assetTypeOf(f.Type),
			Description: f.Tag.Get("description"),
		})
	}
	return fields
}

// assetTypeOf collapses a Go type to string, int, float, bool, or the type's
// own name, with "[]" per slice level. Anything unnamed, such as a map or an
// interface, is an object: metadata is JSON by the time anything reads it.
func assetTypeOf(t reflect.Type) string {
	var brackets string
	for t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		brackets += "[]"
		t = t.Elem()
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// An unnamed type is a map, an interface or a literal struct.
	if t.Name() == "" {
		return "object" + brackets
	}
	return builtinTypeName(t.Name()) + brackets
}

// builtinTypeName maps the builtins and leaves every other name alone, so a
// named type such as BigtableCluster stays recognisable.
func builtinTypeName(name string) string {
	switch name {
	case "string", "bool":
		return name
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "byte", "rune":
		return "int"
	case "float32", "float64":
		return "float"
	}
	return name
}
