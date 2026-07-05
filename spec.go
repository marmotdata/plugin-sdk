package pluginsdk

import (
	"reflect"
	"strconv"
	"strings"
)

type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeInt         FieldType = "int"
	FieldTypeBool        FieldType = "bool"
	FieldTypeSelect      FieldType = "select"
	FieldTypeMultiselect FieldType = "multiselect"
	FieldTypePassword    FieldType = "password"
	FieldTypeObject      FieldType = "object"
)

type ShowWhen struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type ConfigField struct {
	Name        string        `json:"name"`
	Type        FieldType     `json:"type"`
	Label       string        `json:"label"`
	Description string        `json:"description"`
	Required    bool          `json:"required"`
	Default     interface{}   `json:"default,omitempty"`
	Options     []FieldOption `json:"options,omitempty"`
	Validation  *Validation   `json:"validation,omitempty"`
	Sensitive   bool          `json:"sensitive"`
	Placeholder string        `json:"placeholder,omitempty"`
	Fields      []ConfigField `json:"fields,omitempty"`
	IsArray     bool          `json:"is_array,omitempty"`
	ShowWhen    *ShowWhen     `json:"show_when,omitempty"`
	Hidden      bool          `json:"hidden,omitempty"`
}

type FieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Validation struct {
	Pattern string `json:"pattern,omitempty"`
	Min     *int   `json:"min,omitempty"`
	Max     *int   `json:"max,omitempty"`
	MinLen  *int   `json:"min_len,omitempty"`
	MaxLen  *int   `json:"max_len,omitempty"`
}

// GenerateConfigSpec builds a config spec from a plugin config struct
// using reflection over its json/description/validate/... struct tags.
func GenerateConfigSpec(configType interface{}) []ConfigField {
	return generateConfigSpecRecursive(configType, "")
}

func generateConfigSpecRecursive(configType interface{}, prefix string) []ConfigField {
	var fields []ConfigField

	t := reflect.TypeOf(configType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		jsonTag := field.Tag.Get("json")

		// Handle inline embedded structs by recursively processing their fields
		if jsonTag != "" && strings.Contains(jsonTag, "inline") {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}

			if embeddedType.Kind() == reflect.Struct {
				embeddedInstance := reflect.New(embeddedType).Interface()
				embeddedFields := generateConfigSpecRecursive(embeddedInstance, prefix)
				fields = append(fields, embeddedFields...)
			}
			continue
		}

		if field.Anonymous {
			continue
		}

		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		jsonName := strings.Split(jsonTag, ",")[0]

		description := field.Tag.Get("description")
		sensitive := field.Tag.Get("sensitive") == "true"
		hidden := field.Tag.Get("hidden") == "true"
		defaultValue := field.Tag.Get("default")
		validateTag := field.Tag.Get("validate")

		fieldType := inferFieldType(field.Type, sensitive)

		required := false
		if validateTag != "" {
			required = strings.Contains(validateTag, "required")
		}

		label := field.Tag.Get("label")
		if label == "" {
			label = toLabel(jsonName)
		}

		configField := ConfigField{
			Name:        jsonName,
			Type:        fieldType,
			Label:       label,
			Description: description,
			Required:    required,
			Sensitive:   sensitive,
			Hidden:      hidden,
		}

		if defaultValue != "" {
			configField.Default = parseDefault(defaultValue, field.Type)
		}

		// Parse oneof validation and convert to dropdown options
		if validateTag != "" && strings.Contains(validateTag, "oneof=") {
			options := parseOneOfOptions(validateTag)
			if len(options) > 0 {
				configField.Type = FieldTypeSelect
				configField.Options = options
			}
		}

		// Parse show_when tag for conditional field visibility
		showWhenTag := field.Tag.Get("show_when")
		if showWhenTag != "" {
			parts := strings.SplitN(showWhenTag, ":", 2)
			if len(parts) == 2 {
				configField.ShowWhen = &ShowWhen{
					Field: parts[0],
					Value: parts[1],
				}
			}
		}

		// Handle nested structs and arrays of structs
		if fieldType == FieldTypeObject {
			nestedType := field.Type
			if nestedType.Kind() == reflect.Ptr {
				nestedType = nestedType.Elem()
			}

			if nestedType.Kind() == reflect.Slice {
				elemType := nestedType.Elem()
				if elemType.Kind() == reflect.Struct {
					configField.IsArray = true
					nestedInstance := reflect.New(elemType).Interface()
					configField.Fields = generateConfigSpecRecursive(nestedInstance, prefix+jsonName+".")
				}
			} else if nestedType.Kind() == reflect.Struct {
				nestedInstance := reflect.New(nestedType).Interface()
				configField.Fields = generateConfigSpecRecursive(nestedInstance, prefix+jsonName+".")
			}
		}

		fields = append(fields, configField)
	}

	return fields
}

func parseOneOfOptions(validateTag string) []FieldOption {
	parts := strings.Split(validateTag, "oneof=")
	if len(parts) < 2 {
		return nil
	}

	oneofPart := parts[1]
	if idx := strings.Index(oneofPart, ","); idx != -1 {
		oneofPart = oneofPart[:idx]
	}

	values := strings.Fields(oneofPart)
	if len(values) == 0 {
		return nil
	}

	options := make([]FieldOption, 0, len(values))
	for _, value := range values {
		options = append(options, FieldOption{
			Label: toLabel(value),
			Value: value,
		})
	}

	return options
}

func inferFieldType(t reflect.Type, sensitive bool) FieldType {
	if sensitive {
		return FieldTypePassword
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		return FieldTypeString
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return FieldTypeInt
	case reflect.Bool:
		return FieldTypeBool
	case reflect.Slice:
		elemType := t.Elem()
		if elemType.Kind() == reflect.Struct {
			return FieldTypeObject
		}
		return FieldTypeMultiselect
	case reflect.Struct:
		return FieldTypeObject
	default:
		return FieldTypeString
	}
}

func toLabel(fieldName string) string {
	parts := strings.Split(fieldName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[0:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

func parseDefault(value string, t reflect.Type) interface{} {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val, err := strconv.ParseInt(value, 10, 64); err == nil {
			return val
		}
	case reflect.Bool:
		if val, err := strconv.ParseBool(value); err == nil {
			return val
		}
	case reflect.String:
		return value
	}
	return value
}
