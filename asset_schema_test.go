package pluginsdk

import (
	"testing"
	"time"
)

type schemaTestCluster struct {
	ID string `json:"id"`
}

type schemaTestFields struct {
	Path     string              `json:"path" metadata:"path" description:"Path to the file"`
	Count    int64               `json:"count" metadata:"count" description:"How many"`
	Ratio    float64             `json:"ratio" metadata:"ratio"`
	Enabled  bool                `json:"enabled" metadata:"enabled"`
	Keys     []string            `json:"keys" metadata:"keys"`
	Labels   map[string]string   `json:"labels" metadata:"labels"`
	Clusters []schemaTestCluster `json:"clusters" metadata:"clusters"`
	Started  time.Time           `json:"started" metadata:"started"`
	Owner    *string             `json:"owner" metadata:"owner"`
	Extra    any                 `json:"extra" metadata:"extra"`
	Untagged string              `json:"untagged"`
	Excluded string              `json:"excluded" metadata:"-"`
}

func TestAssetSchemaOf(t *testing.T) {
	got := AssetSchemaOf(schemaTestFields{}, "Table", "Tables and views")

	if got.StructName != "schemaTestFields" {
		t.Errorf("StructName = %q, want %q", got.StructName, "schemaTestFields")
	}
	if got.DisplayName != "Table" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Table")
	}
	if got.Description != "Tables and views" {
		t.Errorf("Description = %q, want %q", got.Description, "Tables and views")
	}

	want := []AssetField{
		{Name: "path", Type: "string", Description: "Path to the file"},
		{Name: "count", Type: "int", Description: "How many"},
		{Name: "ratio", Type: "float"},
		{Name: "enabled", Type: "bool"},
		{Name: "keys", Type: "string[]"},
		{Name: "labels", Type: "object"},
		{Name: "clusters", Type: "schemaTestCluster[]"},
		{Name: "started", Type: "Time"},
		{Name: "owner", Type: "string"},
		{Name: "extra", Type: "object"},
	}
	if len(got.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %+v", len(got.Fields), len(want), got.Fields)
	}
	for i, w := range want {
		if got.Fields[i] != w {
			t.Errorf("field %d = %+v, want %+v", i, got.Fields[i], w)
		}
	}
}

func TestAssetSchemaOfAcceptsPointer(t *testing.T) {
	got := AssetSchemaOf(&schemaTestFields{}, "Table", "")
	if got.StructName != "schemaTestFields" {
		t.Errorf("StructName = %q, want %q", got.StructName, "schemaTestFields")
	}
}

// Documentation must never stop a plugin serving, so a bad value degrades to a
// schema with no fields rather than panicking.
func TestAssetSchemaOfWithoutFields(t *testing.T) {
	type undocumented struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name  string
		value any
	}{
		{"not a struct", "sqlite"},
		{"nil", nil},
		{"no metadata tags", undocumented{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssetSchemaOf(tt.value, "Table", "Tables and views")
			if len(got.Fields) != 0 {
				t.Errorf("Fields = %+v, want none", got.Fields)
			}
			if got.DisplayName != "Table" {
				t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Table")
			}
		})
	}
}
