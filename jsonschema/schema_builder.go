// Package jsonschema provides helpers to generate JSON Schema documents
// from Go types. Generated schemas are compatible with JSON Schema draft 2019-09.
// It supports building a root schema and collecting reusable component schemas
// for nested types.
package jsonschema

import (
	"reflect"
	"strings"

	"github.com/fgrzl/json/polymorphic"
)

// Builder builds JSON Schema documents for Go types and optionally
// collects reusable component schemas.
//
// Use a Builder when you need to generate a schema and also obtain
// the component definitions (for example, when producing
// OpenAPI-like components/schemas). Builders are lightweight and
// intended to be created per generation. A Builder is not safe for
// concurrent use: do not call methods on the same Builder instance
// from multiple goroutines simultaneously.
type Builder struct {
	components                 map[string]any
	usesCustomRegisteredSchema bool
}

// NewBuilder returns a new Builder with an initialized components map.
//
// Example:
//
//	b := jsonschema.NewBuilder()
//	root := b.Schema(reflect.TypeOf(MyType{}))
//
// The returned Builder can be reused for additional generations, but
// its internal components map will only be populated when calling
// SchemaWithComponents.
func NewBuilder() *Builder {
	return &Builder{components: make(map[string]any)}
}

// Components returns the map of collected component schemas.
// The map keys are component names and the values are JSON Schema
// objects (map[string]any). The map is populated when calling
// SchemaWithComponents. The returned map is the Builder's internal
// storage and may be mutated by callers; if you need an immutable
// snapshot, copy the map before using it concurrently.
func (b *Builder) Components() map[string]any {
	return b.components
}

// Schema generates a JSON Schema for the provided type and returns
// the schema as a generic map[string]any. This method does not
// populate the Builder's components map — use SchemaWithComponents
// when you also need nested component definitions.
//
// Note: Schema expects a non-nil reflect.Type. Passing a nil
// reflect.Type will cause a panic in the current implementation.
func (b *Builder) Schema(t reflect.Type) map[string]any {
	b.usesCustomRegisteredSchema = false
	if schema, ok := getCachedSchema(t); ok {
		return schema
	}

	schema := b.schemaInternal(t, false)
	cacheSchema(t, schema, !b.usesCustomRegisteredSchema)
	return schema
}

// SchemaWithComponents generates a JSON Schema for the provided type
// and also returns a map of component schemas discovered during
// generation. The first return value is the root schema, the second
// is the components map suitable for placing under
// `#/components/schemas` in OpenAPI-style outputs.
//
// The components map is owned by the Builder and may be mutated by
// subsequent calls to SchemaWithComponents on the same Builder. If
// you need to preserve components between calls, copy the map.
//
// Note: Passing a nil reflect.Type will panic.
func (b *Builder) SchemaWithComponents(t reflect.Type) (map[string]any, map[string]any) {
	b.usesCustomRegisteredSchema = false
	if root, components, ok := getCachedSchemaWithComponents(t); ok {
		b.components = components
		return root, components
	}

	b.components = make(map[string]any)
	root := b.schemaInternalRoot(t, true)
	cacheSchema(t, root, !b.usesCustomRegisteredSchema)
	cacheSchemaWithComponents(t, root, b.components, !b.usesCustomRegisteredSchema)
	return root, b.components
}

func (b *Builder) schemaInternalRoot(t reflect.Type, asRef bool) map[string]any {
	if t == nil {
		panic("reflect.Type must not be nil")
	}
	// For the root type, we want the actual schema, not a reference
	// But we should still generate components for nested types
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if schema, ok := getRegisteredSchema(t); ok {
		if isCustomRegisteredType(t) {
			b.usesCustomRegisteredSchema = true
		}
		return schema
	}

	switch t.Kind() {
	case reflect.Struct:
		// For the root type, always return the actual schema
		schema := b.structSchema(t, asRef)
		// Add root type to components if eligible for refs
		if asRef && t.Name() != "" && isEligibleForRef(t) {
			// If this is a circular reference, add to components
			if b.hasSelfReference(schema, t.Name()) {
				b.components[t.Name()] = schema
			} else {
				// For non-circular types, add to components if no nested components were generated
				// AND the type only contains primitive types (not registered types or anonymous structs)
				if len(b.components) == 0 && b.hasOnlyPrimitiveFields(t) {
					b.components[t.Name()] = schema
				}
			}
		}
		return schema
	case reflect.Slice, reflect.Array:
		return map[string]any{
			TypeKey:  TypeArray,
			ItemsKey: b.schemaInternal(t.Elem(), asRef),
		}
	case reflect.Map:
		return map[string]any{
			TypeKey:                 TypeObject,
			AdditionalPropertiesKey: b.schemaInternal(t.Elem(), asRef),
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

func (b *Builder) schemaInternal(t reflect.Type, asRef bool) map[string]any {
	if t == nil {
		panic("reflect.Type must not be nil")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if schema, ok := getRegisteredSchema(t); ok {
		if isCustomRegisteredType(t) {
			b.usesCustomRegisteredSchema = true
		}
		return schema
	}

	switch t.Kind() {
	case reflect.Struct:
		return b.structSchema(t, asRef)
	case reflect.Slice, reflect.Array:
		return map[string]any{
			TypeKey:  TypeArray,
			ItemsKey: b.schemaInternal(t.Elem(), asRef),
		}
	case reflect.Map:
		return map[string]any{
			TypeKey:                 TypeObject,
			AdditionalPropertiesKey: b.schemaInternal(t.Elem(), asRef),
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

func (b *Builder) structSchema(t reflect.Type, useRef bool) map[string]any {
	schema := map[string]any{TypeKey: TypeObject}
	properties := map[string]any{}
	var required []string

	if p, ok := reflect.New(t).Interface().(polymorphic.Polymorphic); ok {
		schema[IDKey] = p.GetDiscriminator()
	}

	b.populateStructFields(t, useRef, properties, &required)

	schema[PropertiesKey] = properties
	if len(required) > 0 {
		schema[RequiredKey] = required
	}

	return schema
}

func (b *Builder) populateStructFields(t reflect.Type, useRef bool, properties map[string]any, required *[]string) {
	for i := 0; i < t.NumField(); i++ {
		b.populateStructField(t, useRef, properties, required, t.Field(i))
	}
}

func (b *Builder) populateStructField(parentType reflect.Type, useRef bool, properties map[string]any, required *[]string, field reflect.StructField) {
	if field.PkgPath != "" || jsonFieldName(field) == "-" {
		return
	}

	name := jsonFieldName(field)
	if ref := field.Tag.Get(RefKey); ref != "" {
		properties[name] = map[string]any{RefKey: ref}
		return
	}

	ft := field.Type
	ftKind := ft.Kind()
	baseType, baseKind := unwrapSchemaType(ft)

	if field.Anonymous && baseKind == reflect.Struct {
		jsonTag := field.Tag.Get(JSONTag)
		if jsonTag != "" && strings.Contains(jsonTag, "inline") {
			b.mergeEmbeddedStruct(properties, required, baseType)
			return
		}
	}

	if useRef && baseType.Name() != "" && isEligibleForRef(baseType) {
		b.addReferencedStructField(parentType, properties, name, ftKind, baseType, useRef)
		return
	}

	fieldSchema := b.schemaInternal(field.Type, useRef)
	applyFieldTags(field, fieldSchema)

	if field.Tag.Get(RequiredKey) == "true" || field.Tag.Get("binding") == "required" {
		*required = append(*required, name)
	}

	properties[name] = fieldSchema
}

func (b *Builder) mergeEmbeddedStruct(properties map[string]any, required *[]string, embeddedType reflect.Type) {
	embedded := b.schemaInternal(embeddedType, false)

	if props, ok := embedded[PropertiesKey].(map[string]any); ok {
		for k, v := range props {
			if _, exists := properties[k]; !exists {
				properties[k] = v
			}
		}
	}

	switch reqv := embedded[RequiredKey].(type) {
	case []string:
		*required = append(*required, reqv...)
	case []any:
		for _, item := range reqv {
			if s, ok := item.(string); ok {
				*required = append(*required, s)
			}
		}
	}
}

func (b *Builder) addReferencedStructField(parentType reflect.Type, properties map[string]any, name string, ftKind reflect.Kind, baseType reflect.Type, useRef bool) {
	refName := baseType.Name()
	if baseType != parentType {
		if _, exists := b.components[refName]; !exists {
			b.components[refName] = b.schemaInternal(baseType, useRef)
		}
	}

	b.assignReferenceProperty(properties, name, ftKind, refName)
}

func (b *Builder) assignReferenceProperty(properties map[string]any, name string, ftKind reflect.Kind, refName string) {
	ref := map[string]any{RefKey: "#/components/schemas/" + refName}

	switch ftKind {
	case reflect.Slice, reflect.Array:
		properties[name] = map[string]any{
			TypeKey:  TypeArray,
			ItemsKey: ref,
		}
	case reflect.Map:
		properties[name] = map[string]any{
			TypeKey:                 TypeObject,
			AdditionalPropertiesKey: ref,
		}
	default:
		properties[name] = ref
	}
}

func unwrapSchemaType(t reflect.Type) (reflect.Type, reflect.Kind) {
	for {
		kind := t.Kind()
		if kind != reflect.Pointer && kind != reflect.Slice && kind != reflect.Array && kind != reflect.Map {
			return t, kind
		}
		t = t.Elem()
	}
}

func isEligibleForRef(t reflect.Type) bool {
	if t == nil {
		return false
	}

	// unwrap slices, arrays, and pointers
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return false
	}

	_, known := getRegisteredSchema(t)
	return !known
}

// hasSelfReference checks if a schema contains a reference to itself
func (b *Builder) hasSelfReference(schema map[string]any, typeName string) bool {
	return b.containsRefTo(schema, "#/components/schemas/"+typeName)
}

// containsRefTo recursively checks if a schema contains a reference to the given ref
func (b *Builder) containsRefTo(obj any, targetRef string) bool {
	switch v := obj.(type) {
	case map[string]any:
		for key, value := range v {
			if key == "$ref" {
				if ref, ok := value.(string); ok && ref == targetRef {
					return true
				}
			}
			if b.containsRefTo(value, targetRef) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if b.containsRefTo(item, targetRef) {
				return true
			}
		}
	}
	return false
}

// hasOnlyPrimitiveFields checks if a struct type contains only primitive fields (no registered types or complex structs)
func (b *Builder) hasOnlyPrimitiveFields(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" || jsonFieldName(field) == "-" {
			continue
		}

		ft := field.Type
		// Unwrap pointers
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		// Check if this is a primitive type
		switch ft.Kind() {
		case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64, reflect.Bool:
			// This is a primitive type
			continue
		case reflect.Slice, reflect.Array:
			// Check if slice/array of primitives
			elemType := ft.Elem()
			for elemType.Kind() == reflect.Pointer {
				elemType = elemType.Elem()
			}
			switch elemType.Kind() {
			case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64, reflect.Bool:
				continue
			default:
				return false
			}
		default:
			// This is not a primitive type (could be registered type, struct, etc.)
			return false
		}
	}
	return true
}
