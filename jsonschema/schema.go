package jsonschema

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	"github.com/fgrzl/json/polymorphic"
)

// JSON Schema Keys
const (
	TypeKey                 = "type"
	PropertiesKey           = "properties"
	RequiredKey             = "required"
	ItemsKey                = "items"
	AdditionalPropertiesKey = "additionalProperties"
	RefKey                  = "$ref"
	MinimumKey              = "minimum"
	MaximumKey              = "maximum"
	MultipleOfKey           = "multipleOf"
	MinLengthKey            = "minLength"
	MaxLengthKey            = "maxLength"
	PatternKey              = "pattern"
	FormatKey               = "format"
	MinItemsKey             = "minItems"
	MaxItemsKey             = "maxItems"
	UniqueItemsKey          = "uniqueItems"
	EnumKey                 = "enum"
	TitleKey                = "title"
	DescriptionKey          = "description"
	DefaultKey              = "default"
	OneOfKey                = "oneOf"
	AnyOfKey                = "anyOf"
	AllOfKey                = "allOf"
	NotKey                  = "not"
	DataSourceKey           = "dataSource"
	ComponentIDKey          = "componentId"
	DependencyIDKey         = "dependencyId"
	PositionKey             = "position"
	JSONTag                 = "json"
)

// JSON Schema Type Values
const (
	TypeArray   = "array"
	TypeObject  = "object"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeBoolean = "boolean"
	TypeString  = "string"
)

// GenerateSchema produces a JSON Schema as a map for a given Go type.
func GenerateSchema(t reflect.Type) map[string]interface{} {
	// Unwrap pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := make(map[string]interface{})

	// If type implements polymorphic.Polymorphic, set $id from GetDiscriminator
	if t.Kind() == reflect.Struct {
		instance := reflect.New(t).Interface()
		if p, ok := instance.(polymorphic.Polymorphic); ok {
			schema["$id"] = p.GetDiscriminator()
		}
	}

	switch t.Kind() {
	case reflect.Struct:
		schema[TypeKey] = TypeObject
		props := make(map[string]interface{})
		var requiredFields []string

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Get JSON property name
			name := strings.Split(field.Tag.Get(JSONTag), ",")[0]
			if name == "" {
				name = field.Name
			}

			// If a $ref is provided
			if refTag := field.Tag.Get(RefKey); refTag != "" {
				props[name] = map[string]interface{}{RefKey: refTag}
				continue
			}

			fieldSchema := GenerateSchema(field.Type)

			// Numeric constraints
			if typ, ok := fieldSchema[TypeKey].(string); ok && (typ == TypeInteger || typ == TypeNumber) {
				if v := field.Tag.Get(MinimumKey); v != "" {
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						fieldSchema[MinimumKey] = num
					}
				}
				if v := field.Tag.Get(MaximumKey); v != "" {
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						fieldSchema[MaximumKey] = num
					}
				}
				if v := field.Tag.Get(MultipleOfKey); v != "" {
					if num, err := strconv.ParseFloat(v, 64); err == nil {
						fieldSchema[MultipleOfKey] = num
					}
				}
			}

			// String constraints
			if typ, ok := fieldSchema[TypeKey].(string); ok && typ == TypeString {
				if v := field.Tag.Get(MinLengthKey); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema[MinLengthKey] = num
					}
				}
				if v := field.Tag.Get(MaxLengthKey); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema[MaxLengthKey] = num
					}
				}
				if v := field.Tag.Get(PatternKey); v != "" {
					fieldSchema[PatternKey] = v
				}
				if v := field.Tag.Get(FormatKey); v != "" {
					fieldSchema[FormatKey] = v
				}
			}

			// Array constraints
			if typ, ok := fieldSchema[TypeKey].(string); ok && typ == TypeArray {
				if v := field.Tag.Get(MinItemsKey); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema[MinItemsKey] = num
					}
				}
				if v := field.Tag.Get(MaxItemsKey); v != "" {
					if num, err := strconv.Atoi(v); err == nil {
						fieldSchema[MaxItemsKey] = num
					}
				}
				if v := field.Tag.Get(UniqueItemsKey); v != "" {
					fieldSchema[UniqueItemsKey] = (v == "true")
				}
			}

			// Enum support
			if enumTag := field.Tag.Get(EnumKey); enumTag != "" {
				parts := strings.Split(enumTag, ",")
				for i := range parts {
					parts[i] = strings.TrimSpace(parts[i])
				}
				fieldSchema[EnumKey] = parts
			}

			// Custom attributes
			if v := field.Tag.Get(DataSourceKey); v != "" {
				fieldSchema[DataSourceKey] = v
			}
			if v := field.Tag.Get(ComponentIDKey); v != "" {
				fieldSchema[ComponentIDKey] = v
			}
			if v := field.Tag.Get(DependencyIDKey); v != "" {
				fieldSchema[DependencyIDKey] = v
			}
			if v := field.Tag.Get(PositionKey); v != "" {
				if num, err := strconv.Atoi(v); err == nil {
					fieldSchema[PositionKey] = num
				}
			}

			// Metadata
			if v := field.Tag.Get(TitleKey); v != "" {
				fieldSchema[TitleKey] = v
			}
			if v := field.Tag.Get(DescriptionKey); v != "" {
				fieldSchema[DescriptionKey] = v
			}
			if v := field.Tag.Get(DefaultKey); v != "" {
				fieldSchema[DefaultKey] = v
			}

			// Required
			if req := field.Tag.Get(RequiredKey); req == "true" {
				requiredFields = append(requiredFields, name)
			}

			// Nested object: restrict additionalProperties if requested
			if typ, ok := fieldSchema[TypeKey].(string); ok && typ == TypeObject {
				if ap := field.Tag.Get(AdditionalPropertiesKey); ap != "" {
					if ap == "false" {
						fieldSchema[AdditionalPropertiesKey] = false
					} else {
						fieldSchema[AdditionalPropertiesKey] = map[string]interface{}{RefKey: ap}
					}
				}
			}

			props[name] = fieldSchema
		}

		schema[PropertiesKey] = props
		if len(requiredFields) > 0 {
			schema[RequiredKey] = requiredFields
		}
	case reflect.Slice, reflect.Array:
		schema[TypeKey] = TypeArray
		schema[ItemsKey] = GenerateSchema(t.Elem())
	case reflect.Map:
		schema[TypeKey] = TypeObject
		schema[AdditionalPropertiesKey] = GenerateSchema(t.Elem())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema[TypeKey] = TypeInteger
	case reflect.Float32, reflect.Float64:
		schema[TypeKey] = TypeNumber
	case reflect.Bool:
		schema[TypeKey] = TypeBoolean
	case reflect.String:
		schema[TypeKey] = TypeString
	default:
		schema[TypeKey] = TypeString
	}

	return schema
}

// GenerateSchemaRawMessage returns a JSON schema as json.RawMessage.
func GenerateSchemaRawMessage(t reflect.Type) json.RawMessage {
	schema := GenerateSchema(t)
	raw, err := json.Marshal(schema)
	if err != nil {
		slog.Error("error marshalling schema", "error", err)
		return nil
	}
	return raw
}

// SchemaFrom generates a JSON Schema for generic type T.
func SchemaFrom[T any]() json.RawMessage {
	var zero T
	return GenerateSchemaRawMessage(reflect.TypeOf(zero))
}
