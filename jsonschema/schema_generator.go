package jsonschema

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
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

// builtinSchemas returns a new map containing the default type-to-schema mappings.
// Used to initialize and reset the package registry.
func builtinSchemas() map[reflect.Type]map[string]any {
	return map[reflect.Type]map[string]any{
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
}

// registeredSchemas maps Go types to their JSON Schema definitions.
// It is process-wide global state; use ClearRegistry to reset to built-ins only.
var registeredSchemas = builtinSchemas()

var registeredSchemasMu sync.RWMutex

var (
	customRegisteredTypes   = make(map[reflect.Type]struct{})
	customRegisteredTypesMu sync.RWMutex
)

var (
	schemaCache   = make(map[reflect.Type]schemaCacheEntry)
	schemaCacheMu sync.RWMutex

	schemaWithComponentsCache   = make(map[reflect.Type]schemaWithComponentsCacheEntry)
	schemaWithComponentsCacheMu sync.RWMutex

	schemaRawCache   = make(map[reflect.Type]json.RawMessage)
	schemaRawCacheMu sync.RWMutex
)

type schemaWithComponentsCacheEntry struct {
	root       map[string]any
	components map[string]any
	safe       bool
}

type schemaCacheEntry struct {
	schema map[string]any
	safe   bool
}

// rawMessageType is the reflect.Type for json.RawMessage and is used to
// ensure RawMessage is treated as raw JSON (empty schema) rather than a
// byte slice.
var rawMessageType = reflect.TypeOf(json.RawMessage{})

type schemaCloneState struct {
	maps   map[uintptr]map[string]any
	slices map[uintptr][]any
}

func markCustomRegisteredType(t reflect.Type) {
	if t == nil {
		return
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	customRegisteredTypesMu.Lock()
	customRegisteredTypes[t] = struct{}{}
	customRegisteredTypesMu.Unlock()
}

func isCustomRegisteredType(t reflect.Type) bool {
	if t == nil {
		return false
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	customRegisteredTypesMu.RLock()
	_, ok := customRegisteredTypes[t]
	customRegisteredTypesMu.RUnlock()
	return ok
}

func clearCustomRegisteredTypes() {
	customRegisteredTypesMu.Lock()
	customRegisteredTypes = make(map[reflect.Type]struct{})
	customRegisteredTypesMu.Unlock()
}

// getRegisteredSchema attempts to find a registered schema for the
// provided type. It handles a few special cases such as named
// types whose underlying type is []byte (including json.RawMessage).
func getRegisteredSchema(t reflect.Type) (map[string]any, bool) {
	if t == nil {
		return nil, false
	}

	// Normalize pointers
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Special-case json.RawMessage: treat as an empty schema (raw JSON)
	if t == rawMessageType {
		return map[string]any{}, true
	}

	// Check for an exact registered schema first (handles named types
	// like uuid.UUID which may be underlying arrays of bytes).
	registeredSchemasMu.RLock()
	s, ok := registeredSchemas[t]
	registeredSchemasMu.RUnlock()
	if ok {
		return cloneSchemaMap(s), true
	}

	// If this is a slice/array of bytes (anonymous []byte or [N]byte),
	// fall back to the []byte registered schema.
	if (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) && t.Elem().Kind() == reflect.Uint8 {
		registeredSchemasMu.RLock()
		s, ok := registeredSchemas[reflect.TypeOf([]byte{})]
		registeredSchemasMu.RUnlock()
		if ok {
			return cloneSchemaMap(s), true
		}
	}

	return nil, false
}

func cloneSchemaMapFast(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	cloned := make(map[string]any, len(schema))
	for key, value := range schema {
		cloned[key] = cloneSchemaValueFast(value)
	}

	return cloned
}

func cloneSchemaValueFast(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneSchemaMapFast(typed)
	case []any:
		return cloneSchemaSliceFast(typed)
	case []string:
		cloned := make([]string, len(typed))
		copy(cloned, typed)
		return cloned
	default:
		return value
	}
}

func cloneSchemaSliceFast(items []any) []any {
	if items == nil {
		return nil
	}

	cloned := make([]any, len(items))
	for i, item := range items {
		cloned[i] = cloneSchemaValueFast(item)
	}

	return cloned
}

func cloneSchemaMap(schema map[string]any) map[string]any {
	state := &schemaCloneState{
		maps:   make(map[uintptr]map[string]any),
		slices: make(map[uintptr][]any),
	}
	return cloneSchemaMapWithState(schema, state)
}

func cloneSchemaMapWithState(schema map[string]any, state *schemaCloneState) map[string]any {
	if schema == nil {
		return nil
	}

	ptr := reflect.ValueOf(schema).Pointer()
	if cloned, ok := state.maps[ptr]; ok {
		return cloned
	}

	cloned := make(map[string]any, len(schema))
	state.maps[ptr] = cloned
	for key, value := range schema {
		cloned[key] = cloneSchemaValueWithState(value, state)
	}

	return cloned
}

func cloneSchemaValueWithState(value any, state *schemaCloneState) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneSchemaMapWithState(typed, state)
	case []any:
		return cloneSchemaSliceWithState(typed, state)
	case []string:
		cloned := make([]string, len(typed))
		copy(cloned, typed)
		return cloned
	default:
		return value
	}
}

func cloneSchemaSliceWithState(items []any, state *schemaCloneState) []any {
	if items == nil {
		return nil
	}

	ptr := reflect.ValueOf(items).Pointer()
	if cloned, ok := state.slices[ptr]; ok {
		return cloned
	}

	cloned := make([]any, len(items))
	state.slices[ptr] = cloned
	for i, item := range items {
		cloned[i] = cloneSchemaValueWithState(item, state)
	}

	return cloned
}

// GenerateSchema returns the JSON Schema for the provided reflect.Type.
// It is a convenience wrapper around Builder.Schema.
func GenerateSchema(t reflect.Type) map[string]any {
	builder := NewBuilder()
	return builder.Schema(t)
}

// GenerateSchemaWithComponents returns the JSON Schema for the provided reflect.Type
// along with any component schemas discovered during generation.
func GenerateSchemaWithComponents(t reflect.Type) (map[string]any, map[string]any) {
	builder := NewBuilder()
	return builder.SchemaWithComponents(t)
}

func normalizeCacheType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func getCachedSchema(t reflect.Type) (map[string]any, bool) {
	t = normalizeCacheType(t)
	if t == nil {
		return nil, false
	}

	schemaCacheMu.RLock()
	entry, ok := schemaCache[t]
	schemaCacheMu.RUnlock()
	if !ok {
		return nil, false
	}

	if entry.safe {
		return cloneSchemaMapFast(entry.schema), true
	}
	return cloneSchemaMap(entry.schema), true
}

func getCachedSchemaWithComponents(t reflect.Type) (map[string]any, map[string]any, bool) {
	t = normalizeCacheType(t)
	if t == nil {
		return nil, nil, false
	}

	schemaWithComponentsCacheMu.RLock()
	entry, ok := schemaWithComponentsCache[t]
	schemaWithComponentsCacheMu.RUnlock()
	if !ok {
		return nil, nil, false
	}

	if entry.safe {
		return cloneSchemaMapFast(entry.root), cloneSchemaMapFast(entry.components), true
	}
	return cloneSchemaMap(entry.root), cloneSchemaMap(entry.components), true
}

func cacheSchema(t reflect.Type, schema map[string]any, safe bool) {
	t = normalizeCacheType(t)
	if t == nil || schema == nil {
		return
	}

	schemaCacheMu.Lock()
	if safe {
		schemaCache[t] = schemaCacheEntry{schema: cloneSchemaMapFast(schema), safe: true}
	} else {
		schemaCache[t] = schemaCacheEntry{schema: cloneSchemaMap(schema), safe: false}
	}
	schemaCacheMu.Unlock()
}

func getCachedSchemaRawMessage(t reflect.Type) (json.RawMessage, bool) {
	t = normalizeCacheType(t)
	if t == nil {
		return nil, false
	}

	schemaRawCacheMu.RLock()
	raw, ok := schemaRawCache[t]
	schemaRawCacheMu.RUnlock()
	if !ok {
		return nil, false
	}

	return append(json.RawMessage(nil), raw...), true
}

func cacheSchemaRawMessage(t reflect.Type, raw json.RawMessage) {
	t = normalizeCacheType(t)
	if t == nil || raw == nil {
		return
	}

	schemaRawCacheMu.Lock()
	schemaRawCache[t] = append(json.RawMessage(nil), raw...)
	schemaRawCacheMu.Unlock()
}

func cacheSchemaWithComponents(t reflect.Type, root, components map[string]any, safe bool) {
	t = normalizeCacheType(t)
	if t == nil || root == nil {
		return
	}

	schemaWithComponentsCacheMu.Lock()
	if safe {
		schemaWithComponentsCache[t] = schemaWithComponentsCacheEntry{
			root:       cloneSchemaMapFast(root),
			components: cloneSchemaMapFast(components),
			safe:       true,
		}
	} else {
		schemaWithComponentsCache[t] = schemaWithComponentsCacheEntry{
			root:       cloneSchemaMap(root),
			components: cloneSchemaMap(components),
			safe:       false,
		}
	}
	schemaWithComponentsCacheMu.Unlock()
}

func clearSchemaCache() {
	schemaCacheMu.Lock()
	schemaCache = make(map[reflect.Type]schemaCacheEntry)
	schemaCacheMu.Unlock()

	schemaWithComponentsCacheMu.Lock()
	schemaWithComponentsCache = make(map[reflect.Type]schemaWithComponentsCacheEntry)
	schemaWithComponentsCacheMu.Unlock()

	schemaRawCacheMu.Lock()
	schemaRawCache = make(map[reflect.Type]json.RawMessage)
	schemaRawCacheMu.Unlock()
}

// applyFieldTags applies struct tags to a field's JSON Schema.
func applyFieldTags(field reflect.StructField, schema map[string]any) {
	addNumericTags(field, schema)
	addStringTags(field, schema)
	addArrayTags(field, schema)
	applyCommonFieldTags(field, schema)
	applyExtensionTags(field, schema)
	applySchemaKeywordTags(field, schema)
}

func applyCommonFieldTags(field reflect.StructField, schema map[string]any) {
	if val := field.Tag.Get(EnumKey); val != "" {
		parts := strings.Split(val, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		schema[EnumKey] = parts
	}
	if val := field.Tag.Get(TitleKey); val != "" {
		schema[TitleKey] = val
	}
	if val := field.Tag.Get(DescriptionKey); val != "" {
		schema[DescriptionKey] = val
	}
	if val := field.Tag.Get(DefaultKey); val != "" {
		schema[DefaultKey] = val
	}
	if val := field.Tag.Get(AdditionalPropertiesKey); val != "" {
		applyAdditionalPropertiesTag(field, schema, val)
	}
	if val := field.Tag.Get(FormatKey); val != "" {
		schema[FormatKey] = val
	}
}

func applyAdditionalPropertiesTag(field reflect.StructField, schema map[string]any, val string) {
	if val == "false" {
		schema[AdditionalPropertiesKey] = false
		return
	}

	ft := field.Type
	if ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}

	if ft == rawMessageType || len(schema) == 0 {
		schema[TypeKey] = TypeObject
		if strings.HasPrefix(val, "#") {
			schema[AdditionalPropertiesKey] = map[string]any{RefKey: val}
		} else {
			schema[AdditionalPropertiesKey] = map[string]any{}
		}
		return
	}

	if strings.HasPrefix(val, "#") {
		schema[AdditionalPropertiesKey] = map[string]any{RefKey: val}
		return
	}

	schema[AdditionalPropertiesKey] = map[string]any{}
}

func applyExtensionTags(field reflect.StructField, schema map[string]any) {
	for key, val := range parseStructTag(string(field.Tag)) {
		if !strings.HasPrefix(key, "x-") {
			continue
		}
		if _, exists := schema[key]; exists {
			continue
		}
		schema[key] = coerceExtensionTagValue(val)
	}
}

func coerceExtensionTagValue(val string) any {
	trim := strings.TrimSpace(val)
	if len(trim) > 0 && (trim[0] == '[' || trim[0] == '{') {
		var anyVal any
		if err := json.Unmarshal([]byte(trim), &anyVal); err == nil {
			return anyVal
		}
		if trim[0] == '[' && strings.HasSuffix(trim, "]") {
			return parseLooseArrayValue(trim)
		}
	}
	if n, err := strconv.Atoi(trim); err == nil {
		return n
	}
	if b, err := strconv.ParseBool(trim); err == nil {
		return b
	}
	return val
}

func parseLooseArrayValue(trim string) []any {
	inner := strings.TrimSpace(trim[1 : len(trim)-1])
	if inner == "" {
		return []any{}
	}

	parts := strings.Split(inner, ",")
	arr := make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "\"") && strings.HasSuffix(p, "\"") && len(p) >= 2 {
			if unq, err := strconv.Unquote(p); err == nil {
				arr = append(arr, unq)
				continue
			}
			p = p[1 : len(p)-1]
		}
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
	return arr
}

func applySchemaKeywordTags(field reflect.StructField, schema map[string]any) {
	for _, key := range []string{
		ConstKey, ExamplesKey, MinPropertiesKey, MaxPropertiesKey,
		ExclusiveMinimumKey, ExclusiveMaximumKey, PatternPropertiesKey,
		ContainsKey, IfKey, ThenKey, ElseKey, DefsKey, SchemaKey, IDKey,
	} {
		if val := field.Tag.Get(key); val != "" {
			applySchemaKeywordTag(schema, key, val)
		}
	}
}

func applySchemaKeywordTag(schema map[string]any, key, val string) {
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
				return
			}
		}
		schema[key] = val
	case IfKey, ThenKey, ElseKey, ContainsKey:
		if strings.HasPrefix(trim, "#") {
			schema[key] = map[string]any{RefKey: trim}
			return
		}
		if len(trim) > 0 && (trim[0] == '{' || trim[0] == '[') {
			var anyVal any
			if err := json.Unmarshal([]byte(trim), &anyVal); err == nil {
				schema[key] = anyVal
				return
			}
			if trim[0] == '[' && strings.HasSuffix(trim, "]") {
				schema[key] = parseLooseArrayValue(trim)
				return
			}
		}
		schema[key] = val
	default:
		if n, err := strconv.Atoi(trim); err == nil {
			schema[key] = n
			return
		}
		if f, err := strconv.ParseFloat(trim, 64); err == nil {
			schema[key] = f
			return
		}
		if b, err := strconv.ParseBool(trim); err == nil {
			schema[key] = b
			return
		}
		if len(trim) > 0 && (trim[0] == '{' || trim[0] == '[' || trim[0] == '"') {
			var anyVal any
			if err := json.Unmarshal([]byte(trim), &anyVal); err == nil {
				schema[key] = anyVal
				return
			}
		}
		schema[key] = val
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

// RegisterSchema registers a custom JSON Schema for a Go type. The registry is
// process-wide. To restore the default built-in type set (e.g. in tests), call
// ClearRegistry.
func RegisterSchema(t reflect.Type, schema map[string]any) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if schema != nil {
		registeredSchemasMu.Lock()
		registeredSchemas[t] = cloneSchemaMap(schema)
		registeredSchemasMu.Unlock()
		markCustomRegisteredType(t)
		clearSchemaCache()
	}
}

// ClearRegistry resets the type registry to the default built-in mappings and
// removes any custom registrations made via RegisterSchema. Intended for tests
// or process reset.
func ClearRegistry() {
	registeredSchemasMu.Lock()
	registeredSchemas = builtinSchemas()
	registeredSchemasMu.Unlock()
	clearCustomRegisteredTypes()
	clearSchemaCache()
}

// GenerateSchemaRawMessage returns a JSON Schema as a raw JSON message.
// It returns nil if the schema cannot be marshaled.
func GenerateSchemaRawMessage(t reflect.Type) json.RawMessage {
	if raw, ok := getCachedSchemaRawMessage(t); ok {
		return raw
	}

	schema := GenerateSchema(t)
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	cacheSchemaRawMessage(t, raw)
	return raw
}

// SchemaFrom generates a JSON Schema for a generic type T.
func SchemaFrom[T any]() json.RawMessage {
	return GenerateSchemaRawMessage(reflect.TypeFor[T]())
}
