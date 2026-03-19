// Package jsonschema provides helpers to generate JSON Schema documents
// from Go types. Generated schemas are compatible with JSON Schema draft 2019-09.
// It supports building a root schema and collecting reusable component schemas
// for nested types.
package jsonschema

import (
	"log/slog"
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
	components map[string]any
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
	return b.schemaInternal(t, false)
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
	b.components = make(map[string]any)
	root := b.schemaInternalRoot(t, true)
	return root, b.components
}

func (b *Builder) schemaInternalRoot(t reflect.Type, asRef bool) map[string]any {
	if t == nil {
		panic("reflect.Type must not be nil")
	}
	// For the root type, we want the actual schema, not a reference
	// But we should still generate components for nested types
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	name := t.Name()
	if name != "" {
		slog.Info("generate schema", "type", name)
	}

	if schema, ok := getRegisteredSchema(t); ok {
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
	name := t.Name()
	if name != "" {
		slog.Info("generate schema", "type", name)
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if schema, ok := getRegisteredSchema(t); ok {
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
		ftKind := ft.Kind()

		// Unwrap to get base type
		baseType := ft
		baseKind := baseType.Kind()
		for baseKind == reflect.Ptr || baseKind == reflect.Slice || baseKind == reflect.Array || baseKind == reflect.Map {
			baseType = baseType.Elem()
			baseKind = baseType.Kind()
		}

		refName := baseType.Name()

		// If this is an anonymous embedded struct and the json tag contains "inline",
		// merge its properties and required fields into the parent schema rather
		// than emitting a nested property. This supports YAML-style `inline` usage
		// while preserving the Go type definitions.
		if field.Anonymous && baseKind == reflect.Struct {
			jsonTag := field.Tag.Get(JSONTag)
			if jsonTag != "" && strings.Contains(jsonTag, "inline") {
				embedded := b.schemaInternal(baseType, false)
				if props, ok := embedded[PropertiesKey].(map[string]any); ok {
					for k, v := range props {
						if _, exists := properties[k]; !exists {
							properties[k] = v
						}
					}
				}

				// merge required
				if reqv, ok := embedded[RequiredKey].([]string); ok {
					required = append(required, reqv...)
				} else if reqv2, ok := embedded[RequiredKey].([]any); ok {
					for _, it := range reqv2 {
						if s, ok := it.(string); ok {
							required = append(required, s)
						}
					}
				}
				continue
			}
		}

		// Generate component if eligible
		if useRef && refName != "" && isEligibleForRef(baseType) {
			// Check for circular reference - if this field type is the same as our current type
			if baseType == t {
				// This is a circular reference, use a direct reference
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
				continue
			}

			if _, exists := b.components[refName]; !exists {
				refSchema := b.schemaInternal(baseType, useRef)
				b.components[refName] = refSchema
			}

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
			continue
		}

		fieldSchema := b.schemaInternal(field.Type, useRef)
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

func isEligibleForRef(t reflect.Type) bool {
	if t == nil {
		return false
	}

	// unwrap slices, arrays, and pointers
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
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
		for ft.Kind() == reflect.Ptr {
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
			for elemType.Kind() == reflect.Ptr {
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
