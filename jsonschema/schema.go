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

	"github.com/fgrzl/json/polymorphic"
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

// Internal registry of component schemas
var components = map[string]any{}

func RegisterComponent(name string, schema map[string]any) {
	components[name] = schema
}

func GetComponents() map[string]any {
	return components
}

func GenerateSchema(t reflect.Type) map[string]any {
	return generateSchemaInternal(t, true)
}

func GenerateSchemaWithComponents(t reflect.Type) map[string]any {
	components = make(map[string]any)

	root := generateSchemaInternal(t, false)

	rootName := t
	if rootName.Kind() == reflect.Ptr {
		rootName = rootName.Elem()
	}
	refName := rootName.Name()

	RegisterComponent(refName, root)
	return GetComponents()
}

// GenerateSchema creates a JSON Schema for a given Go type.
func generateSchemaInternal(t reflect.Type, inline bool) map[string]any {
	// Dereference pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check for registered schemas
	if schema, ok := knownSchema(t); ok {
		return schema
	}

	// Generate schema based on type kind
	switch t.Kind() {
	case reflect.Struct:
		return generateStructSchema(t, inline)
	case reflect.Slice, reflect.Array:
		return map[string]any{
			TypeKey:  TypeArray,
			ItemsKey: generateSchemaInternal(t.Elem(), inline),
		}
	case reflect.Map:
		return map[string]any{
			TypeKey:                 TypeObject,
			AdditionalPropertiesKey: generateSchemaInternal(t.Elem(), inline),
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{TypeKey: TypeInteger}
	case reflect.Float32, reflect.Float64:
		return map[string]any{TypeKey: TypeNumber}
	case reflect.Bool:
		return map[string]any{TypeKey: TypeBoolean}
	case reflect.String:
		return map[string]any{TypeKey: TypeString}
	default:
		return map[string]any{TypeKey: TypeString}
	}
}

// generateStructSchema generates a JSON Schema for a Go struct type.
func generateStructSchema(t reflect.Type, inline bool) map[string]any {
	schema := map[string]any{TypeKey: TypeObject}
	properties := map[string]any{}
	var required []string

	if p, ok := reflect.New(t).Interface().(polymorphic.Polymorphic); ok {
		schema["$id"] = p.GetDiscriminator()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" || jsonFieldName(field) == "-" {
			continue
		}

		name := jsonFieldName(field)

		if ref := field.Tag.Get(RefKey); ref != "" {
			properties[name] = map[string]any{RefKey: ref}
			continue
		}

		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}

		useRef := !inline &&
			ft.Kind() == reflect.Struct &&
			ft.Name() != "" &&
			ft.PkgPath() != "time" &&
			ft != reflect.TypeOf(uuid.UUID{})

		if useRef {
			if _, ok := knownSchema(ft); !ok {
				refName := ft.Name()
				if _, exists := components[refName]; !exists {
					refSchema := generateStructSchema(ft, inline)

					if ap := field.Tag.Get(AdditionalPropertiesKey); ap != "" {
						if ap == "false" {
							refSchema[AdditionalPropertiesKey] = false
						} else {
							refSchema[AdditionalPropertiesKey] = map[string]any{RefKey: ap}
						}
					}

					RegisterComponent(refName, refSchema)
				}
			}

			properties[name] = map[string]any{RefKey: "#/components/schemas/" + ft.Name()}
			continue
		}

		fieldSchema := generateSchemaInternal(field.Type, inline)
		applyFieldTags(field, fieldSchema)

		if field.Tag.Get(RequiredKey) == "true" || field.Tag.Get("binding") == "required" {
			required = append(required, name)
		}

		properties[name] = fieldSchema
	}

	schema[PropertiesKey] = properties
	if len(required) > 0 {
		schema[RequiredKey] = required
	}
	return schema
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
func addNumericTags(field reflect.StructField, schema map[string]interface{}) {
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

// knownSchema retrieves a registered schema for a given type.
func knownSchema(t reflect.Type) (map[string]any, bool) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	schema, ok := registeredSchemas[t]
	return schema, ok
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
