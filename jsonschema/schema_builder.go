package jsonschema

import (
	"log/slog"
	"reflect"

	"github.com/fgrzl/json/polymorphic"
)

type Builder struct {
	components map[string]any
}

func NewBuilder() *Builder {
	return &Builder{components: make(map[string]any)}
}

func (b *Builder) Components() map[string]any {
	return b.components
}

func (b *Builder) Schema(t reflect.Type) map[string]any {
	return b.schemaInternal(t, false)
}

func (b *Builder) SchemaWithComponents(t reflect.Type) (map[string]any, map[string]any) {
	b.components = make(map[string]any)
	root := b.schemaInternal(t, true)
	return root, b.components
}

func (b *Builder) schemaInternal(t reflect.Type, asRef bool) map[string]any {

	name := t.Name()
	if name != "" {
		slog.Info("generate schema", "type", name)
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if schema, ok := registeredSchemas[t]; ok {
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

		// Generate component if eligible
		if useRef && refName != "" && isEligibleForRef(baseType) {
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

	_, known := registeredSchemas[t]
	return !known
}
