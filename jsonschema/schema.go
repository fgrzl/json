package jsonschema

import (
	"reflect"
	"strconv"
	"strings"
)

// GenerateSchema produces a JSON Schema (as a map) for the given Go type.
// GenerateSchema generates a JSON schema for a given Go type using reflection.
// It supports various JSON schema constraints and annotations through struct tags.
//
// Supported struct tags:
// - json: Specifies the JSON field name.
// - ref: Specifies a schema reference.
// - minimum, maximum, multipleOf: Numeric constraints for integer and number types.
// - minLength, maxLength, pattern, format: String constraints.
// - minItems, maxItems, uniqueItems: Array constraints.
// - enum: Specifies allowed values for the field.
// - title, description, default: Metadata for the field.
// - oneOf, anyOf, allOf, not: Composition keywords for schema definitions.
// - required: Marks the field as required.
// - additionalProperties: Specifies additional properties for object types.
//
// Parameters:
// - t (reflect.Type): The Go type to generate the schema for.
//
// Returns:
// - map[string]interface{}: The generated JSON schema as a map.
func GenerateSchema(t reflect.Type) map[string]interface{} {
	// Unwrap pointer types.
	if t.Kind() == reflect.Ptr {
		return GenerateSchema(t.Elem())
	}

	schema := make(map[string]interface{})
	switch t.Kind() {
	case reflect.Struct:
		schema["type"] = "object"
		props := make(map[string]interface{})
		var requiredFields []string
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			// If a "ref" tag is provided, use that schema reference.
			if refTag := field.Tag.Get("ref"); refTag != "" {
				props[name] = map[string]interface{}{"$ref": refTag}
				continue
			}
			fieldSchema := GenerateSchema(field.Type)

			// Numeric constraints
			if typ, ok := fieldSchema["type"].(string); ok && (typ == "integer" || typ == "number") {
				if v := field.Tag.Get("minimum"); v != "" {
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						fieldSchema["minimum"] = num
					}
				}
				if v := field.Tag.Get("maximum"); v != "" {
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						fieldSchema["maximum"] = num
					}
				}
				if v := field.Tag.Get("multipleOf"); v != "" {
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						fieldSchema["multipleOf"] = num
					}
				}
			}

			// String constraints
			if typ, ok := fieldSchema["type"].(string); ok && typ == "string" {
				if v := field.Tag.Get("minLength"); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema["minLength"] = num
					}
				}
				if v := field.Tag.Get("maxLength"); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema["maxLength"] = num
					}
				}
				if v := field.Tag.Get("pattern"); v != "" {
					fieldSchema["pattern"] = v
				}
				if v := field.Tag.Get("format"); v != "" {
					fieldSchema["format"] = v
				}
			}

			// Array constraints
			if typ, ok := fieldSchema["type"].(string); ok && typ == "array" {
				if v := field.Tag.Get("minItems"); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema["minItems"] = num
					}
				}
				if v := field.Tag.Get("maxItems"); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema["maxItems"] = num
					}
				}
				if v := field.Tag.Get("uniqueItems"); v != "" {
					fieldSchema["uniqueItems"] = (v == "true")
				}
			}

			// Enum support
			if enumTag := field.Tag.Get("enum"); enumTag != "" {
				parts := strings.Split(enumTag, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				fieldSchema["enum"] = parts
			}

			// Metadata
			if v := field.Tag.Get("title"); v != "" {
				fieldSchema["title"] = v
			}
			if v := field.Tag.Get("description"); v != "" {
				fieldSchema["description"] = v
			}
			if v := field.Tag.Get("default"); v != "" {
				fieldSchema["default"] = v
			}

			// Composition keywords
			if v := field.Tag.Get("oneOf"); v != "" {
				parts := strings.Split(v, ",")
				var subschemas []interface{}
				for _, part := range parts {
					subschemas = append(subschemas, map[string]interface{}{"$ref": strings.TrimSpace(part)})
				}
				fieldSchema["oneOf"] = subschemas
			}
			if v := field.Tag.Get("anyOf"); v != "" {
				parts := strings.Split(v, ",")
				var subschemas []interface{}
				for _, part := range parts {
					subschemas = append(subschemas, map[string]interface{}{"$ref": strings.TrimSpace(part)})
				}
				fieldSchema["anyOf"] = subschemas
			}
			if v := field.Tag.Get("allOf"); v != "" {
				parts := strings.Split(v, ",")
				var subschemas []interface{}
				for _, part := range parts {
					subschemas = append(subschemas, map[string]interface{}{"$ref": strings.TrimSpace(part)})
				}
				fieldSchema["allOf"] = subschemas
			}
			if v := field.Tag.Get("not"); v != "" {
				fieldSchema["not"] = map[string]interface{}{"$ref": strings.TrimSpace(v)}
			}

			// Required field
			if req := field.Tag.Get("required"); req == "true" {
				requiredFields = append(requiredFields, name)
			}

			// Additional properties override for objects.
			if typ, ok := fieldSchema["type"].(string); ok && typ == "object" {
				if ap := field.Tag.Get("additionalProperties"); ap != "" {
					if ap == "false" {
						fieldSchema["additionalProperties"] = false
					} else {
						fieldSchema["additionalProperties"] = map[string]interface{}{"$ref": ap}
					}
				}
			}

			props[name] = fieldSchema
		}
		schema["properties"] = props
		if len(requiredFields) > 0 {
			schema["required"] = requiredFields
		}
	case reflect.Slice, reflect.Array:
		schema["type"] = "array"
		schema["items"] = GenerateSchema(t.Elem())
	case reflect.Map:
		schema["type"] = "object"
		// Map keys are assumed to be strings.
		schema["additionalProperties"] = GenerateSchema(t.Elem())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		schema["type"] = "number"
	case reflect.Bool:
		schema["type"] = "boolean"
	case reflect.String:
		schema["type"] = "string"
	default:
		schema["type"] = "string"
	}
	return schema
}
