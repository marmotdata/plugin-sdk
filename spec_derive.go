package pluginsdk

import (
	"fmt"
	"strings"
)

// SpecOption transforms a config spec. Build them with Hide and Override
// and pass them to DeriveSpec.
type SpecOption func(spec []ConfigField) []ConfigField

// FieldOverride mutates a single ConfigField in place. Build them with
// Placeholder, Default, Required or Description and pass them to
// Override.
type FieldOverride func(field *ConfigField)

// DeriveSpec builds a config spec from a plugin config struct and
// applies the given options in order. It is the standard way to build
// an alias plugin's spec, e.g. a Confluent Cloud plugin that reuses
// the Kafka config struct but hides TLS and pins the SASL mechanism:
//
//	spec := pluginsdk.DeriveSpec(kafka.Config{},
//	    pluginsdk.Hide("tls", "authentication.type"),
//	    pluginsdk.Override("bootstrap_servers",
//	        pluginsdk.Placeholder("pkc-xxxxx.confluent.cloud:9092"),
//	    ),
//	)
//
// DeriveSpec panics if any option references a field that does not
// exist in the generated spec. Alias plugins configure this at startup,
// so a bad dotted path is programmer error and should surface loudly
// rather than silently drop the override.
func DeriveSpec(config interface{}, opts ...SpecOption) []ConfigField {
	spec := GenerateConfigSpec(config)
	for _, opt := range opts {
		spec = opt(spec)
	}
	return spec
}

// Hide removes fields from the spec. Nested fields use dot notation
// (e.g. "authentication.type"). Hiding a field that does not exist
// panics.
func Hide(names ...string) SpecOption {
	return func(spec []ConfigField) []ConfigField {
		for _, name := range names {
			if !fieldExists(spec, name, "") {
				panic(fmt.Sprintf("pluginsdk.Hide: field %q does not exist in spec", name))
			}
		}
		return removeFields(spec, names)
	}
}

// Override applies one or more FieldOverrides to a single field.
// Nested fields use dot notation (e.g. "authentication.type").
// Overriding a field that does not exist panics.
func Override(name string, overrides ...FieldOverride) SpecOption {
	return func(spec []ConfigField) []ConfigField {
		if !applyToField(spec, name, "", overrides) {
			panic(fmt.Sprintf("pluginsdk.Override: field %q does not exist in spec", name))
		}
		return spec
	}
}

// Placeholder sets the placeholder text shown in the UI form.
func Placeholder(text string) FieldOverride {
	return func(f *ConfigField) { f.Placeholder = text }
}

// Default sets the default value for the field.
func Default(value interface{}) FieldOverride {
	return func(f *ConfigField) { f.Default = value }
}

// Required marks the field as required, or clears the required flag
// when passed false.
func Required(required bool) FieldOverride {
	return func(f *ConfigField) { f.Required = required }
}

// Description overrides the field's description shown in the UI.
func Description(text string) FieldOverride {
	return func(f *ConfigField) { f.Description = text }
}

// CloneConfigSpec deep-copies a config spec so it can be mutated without
// affecting the original. DeriveSpec starts from a config type and
// produces a fresh spec so most callers do not need this. It is
// exported for advanced use where a spec is derived from another spec
// rather than from a struct.
func CloneConfigSpec(spec []ConfigField) []ConfigField {
	clone := make([]ConfigField, len(spec))
	for i, f := range spec {
		clone[i] = f

		if len(f.Options) > 0 {
			clone[i].Options = make([]FieldOption, len(f.Options))
			copy(clone[i].Options, f.Options)
		}

		if f.Validation != nil {
			v := *f.Validation
			clone[i].Validation = &v
		}

		if f.ShowWhen != nil {
			sw := *f.ShowWhen
			clone[i].ShowWhen = &sw
		}

		if len(f.Fields) > 0 {
			clone[i].Fields = CloneConfigSpec(f.Fields)
		}
	}
	return clone
}

func removeFields(spec []ConfigField, names []string) []ConfigField {
	topLevel := make(map[string]bool)
	nested := make(map[string][]string)

	for _, name := range names {
		if idx := strings.IndexByte(name, '.'); idx != -1 {
			parent := name[:idx]
			child := name[idx+1:]
			nested[parent] = append(nested[parent], child)
		} else {
			topLevel[name] = true
		}
	}

	result := make([]ConfigField, 0, len(spec))
	for _, f := range spec {
		if topLevel[f.Name] {
			continue
		}
		if children, ok := nested[f.Name]; ok && len(f.Fields) > 0 {
			f.Fields = removeFields(f.Fields, children)
		}
		result = append(result, f)
	}
	return result
}

func applyToField(spec []ConfigField, path, prefix string, overrides []FieldOverride) bool {
	for i := range spec {
		key := prefix + spec[i].Name
		if key == path {
			for _, o := range overrides {
				o(&spec[i])
			}
			return true
		}
		if len(spec[i].Fields) > 0 && strings.HasPrefix(path, key+".") {
			if applyToField(spec[i].Fields, path, key+".", overrides) {
				return true
			}
		}
	}
	return false
}

func fieldExists(spec []ConfigField, path, prefix string) bool {
	for _, f := range spec {
		key := prefix + f.Name
		if key == path {
			return true
		}
		if strings.HasPrefix(path, key+".") && fieldExists(f.Fields, path, key+".") {
			return true
		}
	}
	return false
}
