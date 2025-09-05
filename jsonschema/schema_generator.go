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
	ConstKey                = "const"
	ExamplesKey             = "examples"
	MinPropertiesKey        = "minProperties"
	MaxPropertiesKey        = "maxProperties"
	ExclusiveMinimumKey     = "exclusiveMinimum"
	ExclusiveMaximumKey     = "exclusiveMaximum"
	PatternPropertiesKey    = "patternProperties"
	ContainsKey             = "contains"
	IfKey                   = "if"
	ThenKey                 = "then"
	ElseKey                 = "else"
	DefsKey                 = "$defs"
	SchemaKey               = "$schema"
	IDKey                   = "$id"
	OneOfKey                = "oneOf"
	AnyOfKey                = "anyOf"
	AllOfKey                = "allOf"
	NotKey                  = "not"
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

// rawMessageType is the reflect.Type for json.RawMessage and is used to
// ensure RawMessage is treated as raw JSON (empty schema) rather than a
// byte slice.
var rawMessageType = reflect.TypeOf(json.RawMessage{})

// getRegisteredSchema attempts to find a registered schema for the
// provided type. It handles a few special cases such as named
// types whose underlying type is []byte (including json.RawMessage).
func getRegisteredSchema(t reflect.Type) (map[string]any, bool) {
	if t == nil {
		return nil, false
	}

	// Normalize pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Special-case json.RawMessage: treat as an empty schema (raw JSON)
	if t == rawMessageType {
		return map[string]any{}, true
	}

	// Check for an exact registered schema first (handles named types
	// like uuid.UUID which may be underlying arrays of bytes).
	if s, ok := registeredSchemas[t]; ok {
		return s, true
	}

	// If this is a slice/array of bytes (anonymous []byte or [N]byte),
	// fall back to the []byte registered schema.
	if (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && t.Elem().Kind() == reflect.Uint8 {
		if s, ok := registeredSchemas[reflect.TypeOf([]byte{})]; ok {
			return s, true
		}
	}

	return nil, false
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
		// Note: dataSource, componentId, dependencyId and position are now
		// handled via x-* extension tags (for example `x-component:"id"`).
		{TitleKey, func(v string, s map[string]any) { s[TitleKey] = v }},
		{DescriptionKey, func(v string, s map[string]any) { s[DescriptionKey] = v }},
		{DefaultKey, func(v string, s map[string]any) { s[DefaultKey] = v }},
		{AdditionalPropertiesKey, func(v string, s map[string]any) {
			if v == "false" {
				s[AdditionalPropertiesKey] = false
				return
			}

			// Determine if this field is a json.RawMessage so we can
			// coerce it to an object when additionalProperties is used.
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}

			if ft == rawMessageType || len(s) == 0 {
				// For RawMessage or empty schemas, treat the field as an object
				// when additionalProperties is specified.
				s[TypeKey] = TypeObject
				if strings.HasPrefix(v, "#") {
					s[AdditionalPropertiesKey] = map[string]any{RefKey: v}
				} else {
					s[AdditionalPropertiesKey] = map[string]any{}
				}
				return
			}

			// For non-raw fields, do not change the existing type; only set
			// the additionalProperties value (allowing refs or empty schema).
			if strings.HasPrefix(v, "#") {
				s[AdditionalPropertiesKey] = map[string]any{RefKey: v}
			} else {
				s[AdditionalPropertiesKey] = map[string]any{}
			}
		}},
		{FormatKey, func(v string, s map[string]any) { s[FormatKey] = v }},
	} {
		if val := field.Tag.Get(tag.key); val != "" {
			tag.apply(val, schema)
		}
	}

	// Apply custom extension tags: any struct tag key that starts with "x-"
	// will be copied into the schema. We attempt multiple coercions:
	//  - JSON decode if the value looks like an array or object
	//  - integer parse
	//  - boolean parse
	// Otherwise the raw string is used.
	for k, v := range parseStructTag(string(field.Tag)) {
		if strings.HasPrefix(k, "x-") {
			if _, exists := schema[k]; exists {
				continue
			}
			trim := strings.TrimSpace(v)
			if len(trim) > 0 && (trim[0] == '[' || trim[0] == '{') {
				var anyVal any
				if err := json.Unmarshal([]byte(trim), &anyVal); err == nil {
					schema[k] = anyVal
					continue
				}
				// Fallback: support simple unquoted arrays like [a,b]
				if trim[0] == '[' && strings.HasSuffix(trim, "]") {
					inner := strings.TrimSpace(trim[1 : len(trim)-1])
					if inner == "" {
						schema[k] = []any{}
						continue
					}
					parts := strings.Split(inner, ",")
					arr := make([]any, 0, len(parts))
					for _, p := range parts {
						p = strings.TrimSpace(p)
						// strip optional surrounding quotes
						if strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"") && len(p) >= 2 {
							unq, err := strconv.Unquote(p)
							if err == nil {
								arr = append(arr, unq)
								continue
							}
							p = p[1 : len(p)-1]
						}
						arr = append(arr, p)
					}
					schema[k] = arr
					continue
				}
			}
			if n, err := strconv.Atoi(trim); err == nil {
				schema[k] = n
				continue
			}
			if b, err := strconv.ParseBool(trim); err == nil {
				schema[k] = b
				continue
			}
			schema[k] = v
		}
	}

	// Support direct JSON Schema keyword tags (e.g. const, examples, $defs, $schema, $id)
	for _, key := range []string{
		ConstKey, ExamplesKey, MinPropertiesKey, MaxPropertiesKey,
		ExclusiveMinimumKey, ExclusiveMaximumKey, PatternPropertiesKey,
		ContainsKey, IfKey, ThenKey, ElseKey, DefsKey, SchemaKey, IDKey,
	} {
		if val := field.Tag.Get(key); val != "" {
			trim := strings.TrimSpace(val)
			switch key {
			case MinPropertiesKey, MaxPropertiesKey:
				if i, err := strconv.Atoi(trim); err == nil {
					schema[key] = i
				}
			case ExclusiveMinimumKey, ExclusiveMaximumKey:
				if f, err := strconv.ParseFloat(trim, 64); err == nil {
					schema[key] = f
				}
			case ExamplesKey, PatternPropertiesKey, DefsKey:
				if len(trim) > 0 && (trim[0] == '{' || trim[0] == '[') {
					var anyVal any
					if err := json.Unmarshal([]byte(trim), &anyVal); err == nil {
						schema[key] = anyVal
						break
					}
				}
				schema[key] = val
			case IfKey, ThenKey, ElseKey, ContainsKey:
				if strings.HasPrefix(trim, "#") {
					schema[key] = map[string]any{RefKey: trim}
					break
				}
				if len(trim) > 0 && (trim[0] == '{' || trim[0] == '[') {
					var anyVal any
					if err := json.Unmarshal([]byte(trim), &anyVal); err == nil {
						schema[key] = anyVal
						break
					}
					// Fallback: support simple unquoted arrays like [a,b]
					if trim[0] == '[' && strings.HasSuffix(trim, "]") {
						inner := strings.TrimSpace(trim[1 : len(trim)-1])
						if inner == "" {
							schema[key] = []any{}
							break
						}
						parts := strings.Split(inner, ",")
						arr := make([]any, 0, len(parts))
						for _, p := range parts {
							p = strings.TrimSpace(p)
							// strip optional surrounding quotes
							if strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"") && len(p) >= 2 {
								unq, err := strconv.Unquote(p)
								if err == nil {
									arr = append(arr, unq)
									continue
								}
								p = p[1 : len(p)-1]
							}
							// try numeric
							if n, err := strconv.Atoi(p); err == nil {
								arr = append(arr, n)
								continue
							}
							if f, err := strconv.ParseFloat(p, 64); err == nil {
								arr = append(arr, f)
								continue
							}
							if b, err := strconv.ParseBool(p); err == nil {
								arr = append(arr, b)
								continue
							}
							arr = append(arr, p)
						}
						schema[key] = arr
						break
					}
				}
				schema[key] = val
			default:
				// const, $schema, $id: attempt numeric/bool/json coercion, fallback to string
				if n, err := strconv.Atoi(trim); err == nil {
					schema[key] = n
					break
				}
				if f, err := strconv.ParseFloat(trim, 64); err == nil {
					schema[key] = f
					break
				}
				if b, err := strconv.ParseBool(trim); err == nil {
					schema[key] = b
					break
				}
				if len(trim) > 0 && (trim[0] == '{' || trim[0] == '[' || trim[0] == '"') {
					var anyVal any
					if err := json.Unmarshal([]byte(trim), &anyVal); err == nil {
						schema[key] = anyVal
						break
					}
				}
				schema[key] = val
			}
		}
	}
}

// parseStructTag parses a raw struct tag string into a map of key->value.
// It is a lightweight parser that understands the `key:"value"` layout
// used by reflect.StructTag. Returns an empty map on parse errors.
func parseStructTag(raw string) map[string]string {
	res := make(map[string]string)
	i := 0
	n := len(raw)
	for i < n {
		// skip spaces
		for i < n && raw[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}
		// read key
		j := i
		for j < n && raw[j] != ':' && raw[j] != ' ' {
			j++
		}
		if j >= n || raw[j] != ':' {
			break
		}
		key := raw[i:j]
		j++ // skip ':'
		if j >= n || raw[j] != '"' {
			break
		}
		// find end quote, handling escapes
		k := j + 1
		for k < n {
			if raw[k] == '"' {
				// check if escaped
				esc := false
				p := k - 1
				for p >= j+1 && raw[p] == '\\' {
					esc = !esc
					p--
				}
				if !esc {
					break
				}
			}
			k++
		}
		if k >= n {
			break
		}
		quoted := raw[j : k+1]
		val, err := strconv.Unquote(quoted)
		if err != nil {
			// fallback: strip surrounding quotes
			if len(quoted) >= 2 {
				val = quoted[1 : len(quoted)-1]
			} else {
				val = quoted
			}
		}
		res[key] = val
		i = k + 1
	}
	return res
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
