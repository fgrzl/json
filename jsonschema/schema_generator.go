package jsonschema

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JSON Schema constants for keys and types.
const (
	// Schema keywords
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

	// Schema types
	TypeArray   = "array"
	TypeObject  = "object"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeBoolean = "boolean"
	TypeString  = "string"
)

// registeredSchemas maps Go types to their JSON Schema definitions.
var registeredSchemas = map[reflect.Type]map[string]any{
	reflect.TypeOf(uuid.UUID{}):            {TypeKey: TypeString, FormatKey: "uuid"},
	reflect.TypeOf(time.Time{}):            {TypeKey: TypeString, FormatKey: "date-time"},
	reflect.TypeOf([]byte{}):               {TypeKey: TypeString, FormatKey: "byte"},
	reflect.TypeOf((*url.URL)(nil)).Elem(): {TypeKey: TypeString, FormatKey: "uri"},
	reflect.TypeOf(net.IP{}):               {TypeKey: TypeString, FormatKey: "ipv4"},

	// Nullable SQL types
	reflect.TypeOf(sql.NullString{}):  {TypeKey: []any{TypeString, "null"}},
	reflect.TypeOf(sql.NullInt64{}):   {TypeKey: []any{TypeInteger, "null"}},
	reflect.TypeOf(sql.NullBool{}):    {TypeKey: []any{TypeBoolean, "null"}},
	reflect.TypeOf(sql.NullFloat64{}): {TypeKey: []any{TypeNumber, "null"}},
	reflect.TypeOf(sql.NullTime{}): {
		TypeKey:   []any{TypeString, "null"},
		FormatKey: "date-time",
	},
}

func GenerateSchema(t reflect.Type) map[string]any {
	builder := NewBuilder()
	return builder.Schema(t)
}

// GenerateSchema returns the JSON Schema for the provided reflect.Type.
// This is a convenience wrapper around Builder.Schema.
func GenerateSchemaWithComponents(t reflect.Type) (map[string]any, map[string]any) {
	builder := NewBuilder()
	return builder.SchemaWithComponents(t)
}

// applyFieldTags applies struct tags to a field's JSON Schema.
func applyFieldTags(field reflect.StructField, schema map[string]any) {
	addNumericTags(field, schema)
	addStringTags(field, schema)
	addArrayTags(field, schema)

	// Common tags
	for _, tag := range []struct {
		key   string
		apply func(string, map[string]any)
	}{
		{EnumKey, func(v string, s map[string]any) {
			parts := strings.Split(v, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			s[EnumKey] = parts
		}},
		{DataSourceKey, func(v string, s map[string]any) { s[DataSourceKey] = v }},
		{ComponentIDKey, func(v string, s map[string]any) { s[ComponentIDKey] = v }},
		{DependencyIDKey, func(v string, s map[string]any) { s[DependencyIDKey] = v }},
		{PositionKey, func(v string, s map[string]any) {
			if n, err := strconv.Atoi(v); err == nil {
				s[PositionKey] = n
			}
		}},
		{TitleKey, func(v string, s map[string]any) { s[TitleKey] = v }},
		{DescriptionKey, func(v string, s map[string]any) { s[DescriptionKey] = v }},
		{DefaultKey, func(v string, s map[string]any) { s[DefaultKey] = v }},
		{AdditionalPropertiesKey, func(v string, s map[string]any) {
			if v == "false" {
				s[AdditionalPropertiesKey] = false
			} else {
				s[AdditionalPropertiesKey] = map[string]any{RefKey: v}
			}
		}},
		{FormatKey, func(v string, s map[string]any) { s[FormatKey] = v }},
	} {
		if val := field.Tag.Get(tag.key); val != "" {
			tag.apply(val, schema)
		}
	}
}

// addNumericTags applies numeric-specific tags to a schema.
func addNumericTags(field reflect.StructField, schema map[string]any) {
	typ, ok := schema[TypeKey].(string)
	if !ok || (typ != TypeInteger && typ != TypeNumber) {
		return
	}

	for _, tag := range []struct {
		key   string
		apply func(float64)
	}{
		{MinimumKey, func(v float64) { schema[MinimumKey] = v }},
		{MaximumKey, func(v float64) { schema[MaximumKey] = v }},
		{MultipleOfKey, func(v float64) { schema[MultipleOfKey] = v }},
	} {
		if val := field.Tag.Get(tag.key); val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				tag.apply(f)
			}
		}
	}
}

// addStringTags applies string-specific tags to a schema.
func addStringTags(field reflect.StructField, schema map[string]any) {
	if schema[TypeKey] != TypeString {
		return
	}

	for _, tag := range []struct {
		key   string
		apply func(int)
	}{
		{MinLengthKey, func(v int) { schema[MinLengthKey] = v }},
		{MaxLengthKey, func(v int) { schema[MaxLengthKey] = v }},
	} {
		if val := field.Tag.Get(tag.key); val != "" {
			if i, err := strconv.Atoi(val); err == nil {
				tag.apply(i)
			}
		}
	}

	if pattern := field.Tag.Get(PatternKey); pattern != "" {
		schema[PatternKey] = pattern
	}
}

// addArrayTags applies array-specific tags to a schema.
func addArrayTags(field reflect.StructField, schema map[string]any) {
	if schema[TypeKey] != TypeArray {
		return
	}

	for _, tag := range []struct {
		key   string
		apply func(int)
	}{
		{MinItemsKey, func(v int) { schema[MinItemsKey] = v }},
		{MaxItemsKey, func(v int) { schema[MaxItemsKey] = v }},
	} {
		if val := field.Tag.Get(tag.key); val != "" {
			if i, err := strconv.Atoi(val); err == nil {
				tag.apply(i)
			}
		}
	}

	if unique := field.Tag.Get(UniqueItemsKey); unique == "true" {
		schema[UniqueItemsKey] = true
	}
}

// jsonFieldName extracts the JSON field name from a struct field's JSON tag.
func jsonFieldName(f reflect.StructField) string {
	tag := strings.Split(f.Tag.Get(JSONTag), ",")[0]
	if tag == "" {
		return f.Name
	}
	return tag
}

// RegisterSchema registers a custom JSON Schema for a Go type.
func RegisterSchema(t reflect.Type, schema map[string]any) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if schema != nil {
		registeredSchemas[t] = schema
	}
}

// GenerateSchemaRawMessage returns a JSON Schema as a raw JSON message.
// It returns nil if the schema cannot be marshaled.
func GenerateSchemaRawMessage(t reflect.Type) json.RawMessage {
	schema := GenerateSchema(t)
	raw, err := json.Marshal(schema)
	if err != nil {
		slog.Error("error marshalling schema", "error", err)
		return nil
	}
	return raw
}

// SchemaFrom generates a JSON Schema for a generic type T.
func SchemaFrom[T any]() json.RawMessage {
	var zero T
	return GenerateSchemaRawMessage(reflect.TypeOf(zero))
}
