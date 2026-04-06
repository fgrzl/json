// Package jsonschema generates JSON Schema documents from Go types via reflection
// and validates JSON-like data against those schemas.
//
// # Target draft
//
// Generated schemas are compatible with JSON Schema draft 2019-09. The package
// does not set "$schema" on output; callers may add it (e.g.
// https://json-schema.org/draft/2019-09/schema) when validators require it.
//
// # Generation and validation
//
// Use GenerateSchema or Builder to produce a schema from a Go type. Use Validate
// to check decoded JSON (map[string]any, []any, float64, string, bool, nil)
// against a schema. Validation returns nil when valid, or *ErrValidation with
// path and message for each failure. Supported validation keywords: type
// (including nullable), required, properties, items, additionalProperties, enum,
// const, minLength, maxLength, pattern, minimum, maximum, multipleOf,
// exclusiveMinimum, exclusiveMaximum, minItems, maxItems, uniqueItems,
// minProperties, maxProperties, patternProperties, contains,
// $ref (same-document #/$defs/X and #/components/schemas/X, with unresolved
// refs reported as validation errors), allOf, anyOf, oneOf, not, and
// if/then/else.
//
// # Keywords
//
// Emitted keywords include: type, properties, required, items, additionalProperties,
// $ref, format, minimum, maximum, minLength, maxLength, pattern, minItems, maxItems,
// uniqueItems, enum, title, description, default, and struct-tag-driven keywords
// such as const, examples, $defs, if/then/else, minProperties, maxProperties,
// exclusiveMinimum, exclusiveMaximum, patternProperties, contains. References
// use #/components/schemas/ when using SchemaWithComponents.
//
// # Registry
//
// RegisterSchema and the built-in type map (uuid.UUID, time.Time, url.URL, net.IP,
// []byte, json.RawMessage, sql.Null*) are process-wide global state. Tests that
// need a clean slate should call ClearRegistry to restore the default built-in
// set and remove custom registrations.
package jsonschema
