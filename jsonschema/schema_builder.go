package jsonschema

import (
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
	return b.schemaInternal(t, true)
}

func (b *Builder) SchemaWithComponents(t reflect.Type) (map[string]any, map[string]any) {
	b.components = make(map[string]any)
	root := b.schemaInternal(t, false)
	return root, b.components
}

func (b *Builder) schemaInternal(t reflect.Type, inline bool) map[string]any {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if schema, ok := registeredSchemas[t]; ok {
		return schema
	}

	switch t.Kind() {
	case reflect.Struct:
		return b.structSchema(t, inline)
	case reflect.Slice, reflect.Array:
		return map[string]any{
			TypeKey:  TypeArray,
			ItemsKey: b.schemaInternal(t.Elem(), inline),
		}
	case reflect.Map:
		return map[string]any{
			TypeKey:                 TypeObject,
			AdditionalPropertiesKey: b.schemaInternal(t.Elem(), inline),
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

func (b *Builder) structSchema(t reflect.Type, inline bool) map[string]any {
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

		if !inline && isEligibleForRef(ft) {
			refName := ft.Name()
			if _, exists := b.components[refName]; !exists {
				refSchema := b.structSchema(ft, inline)

				if ap := field.Tag.Get(AdditionalPropertiesKey); ap != "" {
					if ap == "false" {
						refSchema[AdditionalPropertiesKey] = false
					} else {
						refSchema[AdditionalPropertiesKey] = map[string]any{RefKey: ap}
					}
				}

				b.components[refName] = refSchema

			}
			properties[name] = map[string]any{RefKey: "#/components/schemas/" + ft.Name()}
			continue
		}

		fieldSchema := b.schemaInternal(field.Type, inline)
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
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	if t.Name() == "" || t.PkgPath() == "" {
		return false
	}
	// Check both value and pointer forms
	if _, known := registeredSchemas[t]; known {
		return false
	}
	return true
}
