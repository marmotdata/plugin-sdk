package pluginsdk

import (
	"encoding/json"
	"reflect"
	"strings"
)

// ApplyDefaults sets config fields to their `default:"..."` struct tag
// value when the corresponding key is absent from raw. It recurses into
// embedded (inline) config sections. Call it from Validate after
// UnmarshalConfig:
//
//	config, err := pluginsdk.UnmarshalConfig[Config](raw)
//	...
//	pluginsdk.ApplyDefaults(config, raw)
//
// Default tag values are parsed as JSON (`default:"true"`,
// `default:"10"`, `default:"[\"a\",\"b\"]"`); values that are not valid
// JSON apply verbatim to string fields (`default:"production"`).
func ApplyDefaults(config interface{}, raw RawConfig) {
	v := reflect.ValueOf(config)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}
	applyDefaults(v, raw)
}

func applyDefaults(v reflect.Value, raw RawConfig) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}

		// Recurse into embedded (inline) config sections.
		if field.Anonymous {
			switch fv.Kind() {
			case reflect.Ptr:
				if !fv.IsNil() && fv.Elem().Kind() == reflect.Struct {
					applyDefaults(fv.Elem(), raw)
				}
			case reflect.Struct:
				applyDefaults(fv, raw)
			}
			continue
		}

		def, ok := field.Tag.Lookup("default")
		if !ok || def == "" {
			continue
		}

		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if key == "" || key == "-" {
			continue
		}

		if _, present := raw[key]; present {
			continue
		}

		setDefault(fv, def)
	}
}

func setDefault(fv reflect.Value, def string) {
	ptr := reflect.New(fv.Type())
	if err := json.Unmarshal([]byte(def), ptr.Interface()); err == nil {
		fv.Set(ptr.Elem())
		return
	}

	// Not valid JSON: apply verbatim to string fields.
	if fv.Kind() == reflect.String {
		fv.SetString(def)
	}
}
