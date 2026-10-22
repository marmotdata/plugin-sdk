package pluginsdk

import "testing"

type groupedShared struct {
	Region string `json:"region"`
	Role   string `json:"role" group:"Authentication"`
}

type groupedConfig struct {
	groupedShared `json:",inline" group:"AWS"`
	Host          string `json:"host" validate:"required"`
	Timeout       int    `json:"timeout" group:"Advanced"`
	Nested        struct {
		Inner string `json:"inner"`
	} `json:"nested" group:"Advanced"`
}

func TestGenerateConfigSpecGroups(t *testing.T) {
	spec := GenerateConfigSpec(groupedConfig{})
	want := map[string]string{"region": "AWS", "role": "Authentication", "host": "", "timeout": "Advanced", "nested": "Advanced"}
	for _, f := range spec {
		if got := f.Group; got != want[f.Name] {
			t.Errorf("field %q: group %q, want %q", f.Name, got, want[f.Name])
		}
		if f.Name == "nested" && f.Fields[0].Group != "" {
			t.Errorf("nested child inherited group %q, want none", f.Fields[0].Group)
		}
	}
}
