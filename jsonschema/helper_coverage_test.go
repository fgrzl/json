package jsonschema

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldValidateBuiltInFormatsGivenFormatConstraints(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		value       string
		wantErr     bool
		wantMessage string
	}{
		{name: "date-time valid", format: "date-time", value: time.Date(2026, time.May, 14, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)},
		{name: "date-time invalid", format: "date-time", value: "not-a-date-time", wantErr: true, wantMessage: "date-time"},
		{name: "uuid valid", format: "uuid", value: "123e4567-e89b-12d3-a456-426614174000"},
		{name: "uuid invalid", format: "uuid", value: "not-a-uuid", wantErr: true, wantMessage: "uuid"},
		{name: "uri valid", format: "uri", value: "https://example.com/path"},
		{name: "uri invalid", format: "uri", value: "not-a-uri", wantErr: true, wantMessage: "uri"},
		{name: "ipv4 valid", format: "ipv4", value: "192.168.1.1"},
		{name: "ipv4 invalid", format: "ipv4", value: "not-an-ip", wantErr: true, wantMessage: "ipv4"},
		{name: "byte valid", format: "byte", value: "aGVsbG8="},
		{name: "byte invalid", format: "byte", value: "not-base64", wantErr: true, wantMessage: "byte"},
		{name: "unsupported format is ignored", format: "email", value: "not-an-email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := &validationPath{}
			errs := make([]ValidationError, 0)

			// Act
			validateFormatConstraint(path, tt.format, tt.value, &errs)

			// Assert
			if tt.wantErr {
				require.Len(t, errs, 1)
				assert.Contains(t, errs[0].Message, tt.wantMessage)
				return
			}

			assert.Empty(t, errs)
		})
	}
}

func TestShouldValidateExplicitTypeGivenTypeShapes(t *testing.T) {
	tests := []struct {
		name    string
		typeVal any
		data    any
		wantErr bool
	}{
		{name: "string type", typeVal: TypeString, data: "hello"},
		{name: "any slice type", typeVal: []any{TypeString, "null"}, data: nil},
		{name: "string slice type", typeVal: []string{TypeString, "null"}, data: "hello"},
		{name: "fallback type rejects mismatch", typeVal: 123, data: 123, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := &validationPath{}
			errs := make([]ValidationError, 0)
			schema := map[string]any{TypeKey: tt.typeVal}

			// Act
			ok := validateExplicitType(map[string]any{}, path, schema, tt.typeVal, tt.data, &errs)

			// Assert
			if tt.wantErr {
				require.False(t, ok)
				require.Len(t, errs, 1)
				return
			}

			require.True(t, ok)
			assert.Empty(t, errs)
		})
	}
}

func TestShouldCompareJSONValuesGivenNestedContainers(t *testing.T) {
	tests := []struct {
		name string
		a    any
		b    any
		want bool
	}{
		{name: "nil values", a: nil, b: nil, want: true},
		{name: "strings", a: "hello", b: "hello", want: true},
		{name: "booleans", a: true, b: true, want: true},
		{name: "floats", a: float64(1.5), b: float64(1.5), want: true},
		{name: "slices", a: []any{"a", float64(1), map[string]any{"b": "c"}}, b: []any{"a", float64(1), map[string]any{"b": "c"}}, want: true},
		{name: "maps", a: map[string]any{"a": []any{float64(1), "x"}}, b: map[string]any{"a": []any{float64(1), "x"}}, want: true},
		{name: "struct fallback", a: struct{ ID int }{ID: 1}, b: struct{ ID int }{ID: 1}, want: true},
		{name: "mismatch", a: map[string]any{"a": true}, b: map[string]any{"a": false}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange

			// Act
			got := deepEqualJSON(tt.a, tt.b)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldGenerateSchemaWithComponentsSafelyGivenCyclicRegisteredSchema(t *testing.T) {
	t.Cleanup(ClearRegistry)

	type CustomType struct{}

	// Arrange
	schema := map[string]any{TypeKey: TypeObject}
	properties := map[string]any{}
	schema[PropertiesKey] = properties
	properties["self"] = schema
	RegisterSchema(reflect.TypeOf(CustomType{}), schema)

	// Act
	firstRoot, firstComponents := GenerateSchemaWithComponents(reflect.TypeOf(CustomType{}))
	firstRoot["title"] = "broken"
	firstRoot[PropertiesKey].(map[string]any)["extra"] = map[string]any{TypeKey: TypeString}
	secondRoot, secondComponents := GenerateSchemaWithComponents(reflect.TypeOf(CustomType{}))

	// Assert
	require.Empty(t, firstComponents)
	require.Empty(t, secondComponents)
	assert.Equal(t, TypeObject, secondRoot[TypeKey])
	assert.NotContains(t, secondRoot, "title")
	assert.NotContains(t, secondRoot[PropertiesKey].(map[string]any), "extra")
	assert.Equal(t, secondRoot, secondRoot[PropertiesKey].(map[string]any)["self"])
}

func TestShouldValidatePatternPropertiesUsingLatestMutableSchemaContent(t *testing.T) {
	// Arrange
	patternProperties := map[string]any{
		"^x-": map[string]any{TypeKey: TypeString},
	}
	schema := map[string]any{
		TypeKey:              TypeObject,
		PatternPropertiesKey: patternProperties,
	}

	// Act
	require.NoError(t, Validate(schema, map[string]any{"x-name": "ok"}))
	delete(patternProperties, "^x-")
	patternProperties["^y-"] = map[string]any{TypeKey: TypeInteger}
	err := Validate(schema, map[string]any{"y-name": "not-an-integer"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/y-name")
}
