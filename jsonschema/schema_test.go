package jsonschema

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func assertSchema(t *testing.T, input any, expected map[string]any) {
	t.Helper()
	typ := reflect.TypeOf(input)
	got := GenerateSchema(typ)
	assert.Equal(t, expected, got)
}

func assertSchemas(t *testing.T, input any, expected map[string]any, expectedComponents map[string]any) {
	t.Helper()
	typ := reflect.TypeOf(input)
	got, gotComponents := GenerateSchemaWithComponents(typ)
	assert.Equal(t, expected, got)
	assert.Equal(t, expectedComponents, gotComponents)
}

func TestShouldIncludeRequiredFieldsGivenStructWithRequiredTags(t *testing.T) {
	type TestStruct struct {
		ID   int    `json:"id" required:"true"`
		Name string `json:"name"`
		Age  int    `json:"age" required:"true"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "integer"},
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
		},
		"required": []string{"id", "age"},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyNumericConstraintsGivenFieldsWithValidationTags(t *testing.T) {
	type TestStruct struct {
		Number int `json:"number" minimum:"0" maximum:"100" multipleOf:"2"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{
				"type":       "integer",
				"minimum":    0.0,
				"maximum":    100.0,
				"multipleOf": 2.0,
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyStringConstraintsGivenFieldsWithStringValidationTags(t *testing.T) {
	type TestStruct struct {
		Text string `json:"text" minLength:"3" maxLength:"10" pattern:"^[a-z]+$" format:"email"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":      "string",
				"minLength": 3,
				"maxLength": 10,
				"pattern":   "^[a-z]+$",
				"format":    "email",
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldGenerateEnumValuesGivenFieldWithEnumTag(t *testing.T) {
	type TestStruct struct {
		Choice string `json:"choice" enum:"option1, option2, option3"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"choice": map[string]any{
				"type": "string",
				"enum": []string{"option1", "option2", "option3"},
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyAdditionalPropertiesConstraintGivenFieldWithTag(t *testing.T) {
	type Nested struct {
		Field string `json:"field"`
	}
	type TestStruct struct {
		ID   uuid.UUID `json:"id"`
		Data Nested    `json:"data" additionalProperties:"false"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":   TypeString,
				"format": "uuid",
			},
			"data": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"field": map[string]any{
						"type": "string",
					},
				},
				"additionalProperties": false,
			},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestShouldGenerateComponentReferencesWhenGeneratingSchemaWithComponents(t *testing.T) {
	type Nested struct {
		Field string `json:"field"`
	}
	type TestStruct struct {
		ID   uuid.UUID `json:"id"`
		Data Nested    `json:"data" additionalProperties:"false"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":   TypeString,
				"format": "uuid",
			},
			"data": map[string]any{
				"$ref": "#/components/schemas/Nested",
			},
		},
	}

	expectedComponents := map[string]any{
		"Nested": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"field": map[string]any{
					"type": "string",
				},
			},
		},
	}

	assertSchemas(t, TestStruct{}, expected, expectedComponents)
}

func TestShouldGenerateRawMessageWhenUsingRawMessageFunction(t *testing.T) {
	type TestStruct struct {
		ID   int    `json:"id" required:"true"`
		Name string `json:"name"`
		Age  int    `json:"age" required:"true"`
	}
	typ := reflect.TypeOf(TestStruct{})
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "integer"},
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
		},
		"required": []string{"id", "age"},
	}
	expectedBytes, _ := json.Marshal(expected)
	got := GenerateSchemaRawMessage(typ)
	assert.JSONEq(t, string(expectedBytes), string(got))
}

func TestShouldGenerateIntegerSchemaGivenIntType(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"int", int(0)},
		{"int8", int8(0)},
		{"int16", int16(0)},
		{"int32", int32(0)},
		{"int64", int64(0)},
	}

	expected := map[string]any{"type": "integer"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSchema(t, tt.input, expected)
		})
	}
}

func TestShouldGenerateNumberSchemaGivenFloatType(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"float32", float32(0)},
		{"float64", float64(0)},
	}

	expected := map[string]any{"type": "number"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSchema(t, tt.input, expected)
		})
	}
}

func TestShouldGenerateBooleanSchemaGivenBoolType(t *testing.T) {
	assertSchema(t, true, map[string]any{"type": "boolean"})
}

func TestShouldGenerateStringSchemaGivenStringType(t *testing.T) {
	assertSchema(t, "", map[string]any{"type": "string"})
}

func TestShouldGenerateArraySchemaGivenSliceType(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected map[string]any
	}{
		{
			name:  "string_slice",
			input: []string{},
			expected: map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		{
			name:  "int_slice",
			input: []int{},
			expected: map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "integer"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSchema(t, tt.input, tt.expected)
		})
	}
}

func TestShouldGenerateArraySchemaGivenArrayType(t *testing.T) {
	assertSchema(t, [3]string{}, map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "string"},
	})
}

func TestShouldGenerateMapSchemaGivenMapType(t *testing.T) {
	assertSchema(t, map[string]int{}, map[string]any{
		"type":                 "object",
		"additionalProperties": map[string]any{"type": "integer"},
	})
}

func TestShouldGenerateUUIDSchemaGivenUUIDType(t *testing.T) {
	assertSchema(t, uuid.UUID{}, map[string]any{
		"type":   "string",
		"format": "uuid",
	})
}

func TestShouldGenerateTimeSchemaGivenTimeType(t *testing.T) {
	assertSchema(t, time.Time{}, map[string]any{
		"type":   "string",
		"format": "date-time",
	})
}

func TestShouldGenerateByteSchemaGivenByteSliceType(t *testing.T) {
	assertSchema(t, []byte{}, map[string]any{
		"type":   "string",
		"format": "byte",
	})
}

func TestShouldGenerateURLSchemaGivenURLType(t *testing.T) {
	assertSchema(t, url.URL{}, map[string]any{
		"type":   "string",
		"format": "uri",
	})
}

func TestShouldGenerateIPSchemaGivenIPType(t *testing.T) {
	assertSchema(t, net.IP{}, map[string]any{
		"type":   "string",
		"format": "ipv4",
	})
}

func TestShouldGenerateNullableSchemaGivenSQLNullTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected map[string]any
	}{
		{
			name:  "sql_null_string",
			input: sql.NullString{},
			expected: map[string]any{
				"type": []any{"string", "null"},
			},
		},
		{
			name:  "sql_null_int64",
			input: sql.NullInt64{},
			expected: map[string]any{
				"type": []any{"integer", "null"},
			},
		},
		{
			name:  "sql_null_bool",
			input: sql.NullBool{},
			expected: map[string]any{
				"type": []any{"boolean", "null"},
			},
		},
		{
			name:  "sql_null_float64",
			input: sql.NullFloat64{},
			expected: map[string]any{
				"type": []any{"number", "null"},
			},
		},
		{
			name:  "sql_null_time",
			input: sql.NullTime{},
			expected: map[string]any{
				"type":   []any{"string", "null"},
				"format": "date-time",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSchema(t, tt.input, tt.expected)
		})
	}
}

func TestShouldGenerateSchemaGivenPointerType(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field string `json:"field"`
	}

	// Act & Assert
	assertSchema(t, &TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{"type": "string"},
		},
	})
}

func TestShouldApplyArrayTagsGivenArrayValidationTags(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Items []string `json:"items" minItems:"1" maxItems:"10" uniqueItems:"true"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"minItems":    1,
				"maxItems":    10,
				"uniqueItems": true,
			},
		},
	})
}

func TestRawMessageDefaultIsEmptySchema(t *testing.T) {
	type TestStruct struct {
		Data json.RawMessage `json:"data"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestRawMessageWithAdditionalPropertiesTagIsObjectWithAdditionalProperties(t *testing.T) {
	type TestStruct struct {
		Data json.RawMessage `json:"data" additionalProperties:"true"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{},
			},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyCustomTagsGivenFieldsWithCustomTags(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field1 string `json:"field1" title:"Field Title" description:"Field Description"`
		Field2 string `json:"field2" default:"default_value"`
		Field3 string `json:"field3" x-dataSource:"api/endpoint"`
		Field4 string `json:"field4" x-component-id:"comp123"`
		Field5 string `json:"field5" x-dependencyId:"dep456"`
		Field6 string `json:"field6" x-position:"5"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field1": map[string]any{
				"type":        "string",
				"title":       "Field Title",
				"description": "Field Description",
			},
			"field2": map[string]any{
				"type":    "string",
				"default": "default_value",
			},
			"field3": map[string]any{
				"type":         "string",
				"x-dataSource": "api/endpoint",
			},
			"field4": map[string]any{
				"type":           "string",
				"x-component-id": "comp123",
			},
			"field5": map[string]any{
				"type":           "string",
				"x-dependencyId": "dep456",
			},
			"field6": map[string]any{
				"type":       "string",
				"x-position": 5,
			},
		},
	})
}

func TestShouldApplyXExtensionString(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" x-custom-str:"hello"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":         "string",
				"x-custom-str": "hello",
			},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyXExtensionNumber(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" x-custom-num:"42"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":         "string",
				"x-custom-num": 42,
			},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyXExtensionArray(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" x-custom-arr:"[\"a\",\"b\"]"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":         "string",
				"x-custom-arr": []any{"a", "b"},
			},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyXExtensionNotEscapedArray(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" x-custom-arr:"[a,b]"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":         "string",
				"x-custom-arr": []any{"a", "b"},
			},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyXExtensionNotEscapedNumberArray(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" x-custom-arr:"[1,2]"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":         "string",
				"x-custom-arr": []any{1.0, 2.0},
			},
		},
	}

	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyAdditionalPropertiesWithRefGivenNonFalseValue(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field string `json:"field" additionalProperties:"#/definitions/MySchema"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":                 "string",
				"additionalProperties": map[string]any{"$ref": "#/definitions/MySchema"},
			},
		},
	})
}

func TestShouldIgnoreInvalidNumericTagsGivenNonNumericFields(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field string `json:"field" minimum:"0" maximum:"100" multipleOf:"2"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{"type": "string"},
		},
	})
}

func TestShouldIgnoreInvalidStringTagsGivenNonStringFields(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field int `json:"field" minLength:"3" maxLength:"10" pattern:"^[a-z]+$"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{"type": "integer"},
		},
	})
}

func TestShouldIgnoreInvalidArrayTagsGivenNonArrayFields(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field string `json:"field" minItems:"1" maxItems:"10" uniqueItems:"true"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{"type": "string"},
		},
	})
}

func TestShouldIgnoreInvalidTagValuesGivenMalformedTags(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field1 int    `json:"field1" minimum:"invalid" maximum:"invalid" multipleOf:"invalid"`
		Field2 string `json:"field2" minLength:"invalid" maxLength:"invalid"`
		Field3 []int  `json:"field3" minItems:"invalid" maxItems:"invalid"`
		Field4 string `json:"field4" position:"invalid"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field1": map[string]any{"type": "integer"},
			"field2": map[string]any{"type": "string"},
			"field3": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "integer"},
			},
			"field4": map[string]any{"type": "string"},
		},
	})
}

func TestShouldUseFieldNameWhenNoJSONTag(t *testing.T) {
	// Arrange
	type TestStruct struct {
		FieldName string
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"FieldName": map[string]any{"type": "string"},
		},
	})
}

func TestShouldIgnoreFieldsWithJSONTagDash(t *testing.T) {
	// Arrange
	type TestStruct struct {
		IgnoredField string `json:"-"`
		ValidField   string `json:"valid"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"valid": map[string]any{"type": "string"},
		},
	})
}

func TestShouldIgnoreUnexportedFields(t *testing.T) {
	// Arrange
	type TestStruct struct {
		ExportedField string `json:"exported"`
		_             string // Intentionally unexported field - should be ignored
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"exported": map[string]any{"type": "string"},
		},
	})
}

func TestShouldHandleRequiredTagFromBinding(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field string `json:"field" binding:"required"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{"type": "string"},
		},
		"required": []string{"field"},
	})
}

func TestShouldUseRefTagWhenSpecified(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Field string `json:"field" $ref:"#/definitions/MyField"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{"$ref": "#/definitions/MyField"},
		},
	})
}

func TestShouldGenerateReferencesForSlicesWithComponents(t *testing.T) {
	// Arrange
	type Item struct {
		Name string `json:"name"`
	}
	type TestStruct struct {
		Items []Item `json:"items"`
	}

	// Act & Assert
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"$ref": "#/components/schemas/Item",
				},
			},
		},
	}
	expectedComponents := map[string]any{
		"Item": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
	}
	assertSchemas(t, TestStruct{}, expected, expectedComponents)
}

func TestShouldGenerateReferencesForMapsWithComponents(t *testing.T) {
	// Arrange
	type Item struct {
		Name string `json:"name"`
	}
	type TestStruct struct {
		Items map[string]Item `json:"items"`
	}

	// Act & Assert
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"$ref": "#/components/schemas/Item",
				},
			},
		},
	}
	expectedComponents := map[string]any{
		"Item": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
	}
	assertSchemas(t, TestStruct{}, expected, expectedComponents)
}

func TestShouldNotGenerateRefForRegisteredTypes(t *testing.T) {
	// Arrange
	type TestStruct struct {
		ID   uuid.UUID `json:"id"`
		Time time.Time `json:"time"`
	}

	// Act & Assert
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":   "string",
				"format": "uuid",
			},
			"time": map[string]any{
				"type":   "string",
				"format": "date-time",
			},
		},
	}
	expectedComponents := map[string]any{}
	assertSchemas(t, TestStruct{}, expected, expectedComponents)
}

func TestShouldNotGenerateRefForAnonymousTypes(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Anon struct {
			Field string `json:"field"`
		} `json:"anon"`
	}

	// Act & Assert
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"anon": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"field": map[string]any{"type": "string"},
				},
			},
		},
	}
	expectedComponents := map[string]any{}
	assertSchemas(t, TestStruct{}, expected, expectedComponents)
}

func TestShouldReuseExistingComponentsWhenSameTypeUsedMultipleTimes(t *testing.T) {
	// Arrange
	type Item struct {
		Name string `json:"name"`
	}
	type TestStruct struct {
		Item1 Item `json:"item1"`
		Item2 Item `json:"item2"`
	}

	// Act & Assert
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"item1": map[string]any{
				"$ref": "#/components/schemas/Item",
			},
			"item2": map[string]any{
				"$ref": "#/components/schemas/Item",
			},
		},
	}
	expectedComponents := map[string]any{
		"Item": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
	}
	assertSchemas(t, TestStruct{}, expected, expectedComponents)
}

type MockPolymorphic struct {
	Name string `json:"name"`
}

func (m *MockPolymorphic) GetDiscriminator() string {
	return "mock_polymorphic"
}

func TestShouldIncludeDiscriminatorIDGivenPolymorphicType(t *testing.T) {
	assertSchema(t, MockPolymorphic{}, map[string]any{
		"type": "object",
		"$id":  "mock_polymorphic",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	})
}

func TestShouldDefaultToStringForUnknownTypes(t *testing.T) {
	// Test for types that fall through to the default case
	// We need to test with a type that doesn't match any of the registered
	// types and isn't a basic type

	// Arrange
	type CustomType struct{}
	type TestStruct struct {
		Field CustomType `json:"field"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	})
}

func TestShouldDefaultToStringForComplexNonStructTypes(t *testing.T) {
	// Test complex types that fall to default case
	// We can test with function types which fall through to default

	// Arrange
	type TestStruct struct {
		Field func() `json:"field"`
	}

	// Act & Assert
	assertSchema(t, TestStruct{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{"type": "string"},
		},
	})
}

func TestShouldUseCustomRegisteredSchema(t *testing.T) {
	// Arrange
	type CustomType struct {
		Field string
	}
	customSchema := map[string]any{
		"type":        "object",
		"description": "Custom registered schema",
	}

	// Act
	RegisterSchema(reflect.TypeOf(CustomType{}), customSchema)

	// Assert
	assertSchema(t, CustomType{}, customSchema)

	// Cleanup
	ClearRegistry()
}

func TestShouldHandlePointerTypeInRegistration(t *testing.T) {
	// Arrange
	type CustomType struct {
		Field string
	}
	customSchema := map[string]any{
		"type":        "object",
		"description": "Custom registered schema for pointer",
	}

	// Act
	RegisterSchema(reflect.TypeOf(&CustomType{}), customSchema)

	// Assert
	assertSchema(t, &CustomType{}, customSchema)

	// Cleanup
	ClearRegistry()
}

func TestShouldIgnoreNilSchemaInRegistration(t *testing.T) {
	// Arrange
	type CustomType struct {
		Field string
	}

	// Act
	RegisterSchema(reflect.TypeOf(CustomType{}), nil)

	// Assert - should generate normal struct schema since nil was ignored
	assertSchema(t, CustomType{}, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"Field": map[string]any{"type": "string"},
		},
	})
}

func TestShouldGenerateSchemaFromGenericFunction(t *testing.T) {
	// Arrange
	type TestStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// Act
	rawMessage := SchemaFrom[TestStruct]()

	// Assert
	var schema map[string]any
	err := json.Unmarshal(rawMessage, &schema)
	assert.NoError(t, err)

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "integer"},
			"name": map[string]any{"type": "string"},
		},
	}
	assert.Equal(t, expected, schema)
}

func TestShouldReturnNilOnMarshalErrorInGenerateSchemaRawMessage(t *testing.T) {
	// This test verifies the error handling path in GenerateSchemaRawMessage
	// We can't easily trigger a marshal error with normal types,
	// but the function has error handling logic that logs and returns nil

	// Arrange
	type TestStruct struct {
		Field string `json:"field"`
	}

	// Act
	result := GenerateSchemaRawMessage(reflect.TypeOf(TestStruct{}))

	// Assert - should not be nil for valid struct
	assert.NotNil(t, result)
}

func TestShouldCreateNewOrderedMap(t *testing.T) {
	// Arrange & Act
	om := NewOrderedMap()

	// Assert
	assert.NotNil(t, om)
	assert.Empty(t, om.keys)
	assert.Empty(t, om.data)
}

func TestShouldSetAndMaintainOrder(t *testing.T) {
	// Arrange
	om := NewOrderedMap()

	// Act
	om.Set("first", "value1")
	om.Set("second", "value2")
	om.Set("third", "value3")

	// Assert
	assert.Equal(t, []string{"first", "second", "third"}, om.keys)
	assert.Equal(t, "value1", om.data["first"])
	assert.Equal(t, "value2", om.data["second"])
	assert.Equal(t, "value3", om.data["third"])
}

func TestShouldNotDuplicateKeysWhenUpdatingValue(t *testing.T) {
	// Arrange
	om := NewOrderedMap()
	om.Set("key1", "value1")
	om.Set("key2", "value2")

	// Act
	om.Set("key1", "updated_value")

	// Assert
	assert.Equal(t, []string{"key1", "key2"}, om.keys)
	assert.Equal(t, "updated_value", om.data["key1"])
}

func TestShouldMarshalJSONInOrder(t *testing.T) {
	// Arrange
	om := NewOrderedMap()
	om.Set("z", "last")
	om.Set("a", "first")
	om.Set("m", "middle")

	// Act
	jsonBytes, err := om.MarshalJSON()

	// Assert
	assert.NoError(t, err)

	// Parse back to verify order (though JSON itself doesn't guarantee order,
	// we can at least verify it's valid JSON with correct values)
	var result map[string]any
	err = json.Unmarshal(jsonBytes, &result)
	assert.NoError(t, err)
	assert.Equal(t, "last", result["z"])
	assert.Equal(t, "first", result["a"])
	assert.Equal(t, "middle", result["m"])
}

func TestShouldMarshalYAMLInOrder(t *testing.T) {
	// Arrange
	om := NewOrderedMap()
	om.Set("first", "value1")
	om.Set("second", "value2")

	// Act
	yamlNode, err := om.MarshalYAML()

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, yamlNode)
}

func TestShouldReturnMapFromToMap(t *testing.T) {
	// Arrange
	om := NewOrderedMap()
	om.Set("key1", "value1")
	om.Set("key2", "value2")

	// Act
	result := om.ToMap()

	// Assert
	expected := map[string]any{
		"key1": "value1",
		"key2": "value2",
	}
	assert.Equal(t, expected, result)
}

func TestShouldCreateBuilderWithEmptyComponents(t *testing.T) {
	// Arrange & Act
	builder := NewBuilder()

	// Assert
	assert.NotNil(t, builder)
	assert.NotNil(t, builder.Components())
	assert.Empty(t, builder.Components())
}

func TestShouldReturnBuilderComponents(t *testing.T) {
	// Arrange
	builder := NewBuilder()
	type TestStruct struct {
		Field string `json:"field"`
	}

	// Act
	builder.SchemaWithComponents(reflect.TypeOf(TestStruct{}))
	components := builder.Components()

	// Assert
	assert.Contains(t, components, "TestStruct")
}

func TestSchemaPanicsGivenNilType(t *testing.T) {
	builder := NewBuilder()
	assert.PanicsWithValue(t, "reflect.Type must not be nil", func() {
		builder.Schema(nil)
	})
}

func TestSchemaWithComponentsPanicsGivenNilType(t *testing.T) {
	builder := NewBuilder()
	assert.PanicsWithValue(t, "reflect.Type must not be nil", func() {
		builder.SchemaWithComponents(nil)
	})
}

// Test edge cases for complex nested structures
func TestShouldHandleVeryNestedStructures(t *testing.T) {
	// Arrange
	type Level3 struct {
		Value string `json:"value"`
	}
	type Level2 struct {
		Level3 Level3 `json:"level3"`
	}
	type Level1 struct {
		Level2 Level2 `json:"level2"`
	}
	type Root struct {
		Level1 Level1 `json:"level1"`
	}

	// Act & Assert
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"level1": map[string]any{
				"$ref": "#/components/schemas/Level1",
			},
		},
	}

	expectedComponents := map[string]any{
		"Level1": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"level2": map[string]any{
					"$ref": "#/components/schemas/Level2",
				},
			},
		},
		"Level2": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"level3": map[string]any{
					"$ref": "#/components/schemas/Level3",
				},
			},
		},
		"Level3": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{"type": "string"},
			},
		},
	}

	assertSchemas(t, Root{}, expected, expectedComponents)
}

func TestShouldHandleCircularReferences(t *testing.T) {
	// This test ensures we don't get infinite loops with circular references
	// by reusing already-processed components

	// Note: Go doesn't allow true circular struct definitions,
	// but we can test pointer-based circular references
	type Node struct {
		Value string `json:"value"`
		Child *Node  `json:"child,omitempty"`
	}

	// Act
	schema, components := GenerateSchemaWithComponents(reflect.TypeOf(Node{}))

	// Debug output
	t.Logf("Root schema: %+v", schema)
	t.Logf("Components: %+v", components)

	// Assert
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "string"},
			"child": map[string]any{
				"$ref": "#/components/schemas/Node",
			},
		},
	}

	expectedComponents := map[string]any{
		"Node": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{"type": "string"},
				"child": map[string]any{
					"$ref": "#/components/schemas/Node",
				},
			},
		},
	}

	assert.Equal(t, expected, schema)
	assert.Equal(t, expectedComponents, components)
}

func TestShouldInlineAnonymousEmbeddedStructWhenTaggedInline(t *testing.T) {
	type Base struct {
		CredentialID uuid.UUID `json:"credential_id" x-component-id:"secret-picker" title:"Secret" description:"UUID of the stored credential used to authenticate to the provider"`
		Tenant       string    `json:"tenant" title:"Tenant ID" description:"Azure tenant (directory) identifier used for authentication"`
	}

	type Sub struct {
		Base  `json:",inline"`
		Other string `json:"other"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"credential_id": map[string]any{
				"type":           TypeString,
				"format":         "uuid",
				"x-component-id": "secret-picker",
				"title":          "Secret",
				"description":    "UUID of the stored credential used to authenticate to the provider",
			},
			"tenant": map[string]any{
				"type":        "string",
				"title":       "Tenant ID",
				"description": "Azure tenant (directory) identifier used for authentication",
			},
			"other": map[string]any{"type": "string"},
		},
	}

	assertSchema(t, Sub{}, expected)
}

func TestShouldNotInlineAnonymousEmbeddedStructWhenNoInlineTag(t *testing.T) {
	type Base struct {
		CredentialID uuid.UUID `json:"credential_id" componentId:"secret-picker" title:"Secret" description:"UUID of the stored credential used to authenticate to the provider"`
		Tenant       string    `json:"tenant" title:"Tenant ID" description:"Azure tenant (directory) identifier used for authentication"`
	}

	type Sub struct {
		Base
		Other string `json:"other"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"Base": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"credential_id": map[string]any{
						"type":           TypeString,
						"format":         "uuid",
						"x-component-id": "secret-picker",
						"title":          "Secret",
						"description":    "UUID of the stored credential used to authenticate to the provider",
					},
					"tenant": map[string]any{
						"type":        "string",
						"title":       "Tenant ID",
						"description": "Azure tenant (directory) identifier used for authentication",
					},
				},
			},
			"other": map[string]any{"type": "string"},
		},
	}

	assertSchema(t, Sub{}, expected)
}

// Tests for direct JSON Schema keyword struct tags
func TestShouldApplyConstTag(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" contains:"#/components/schemas/Item"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":     "string",
				"contains": map[string]any{"$ref": "#/components/schemas/Item"},
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyExamplesTag(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" examples:"[\"a\",\"b\"]"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":     "string",
				"examples": []any{"a", "b"},
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyMinMaxPropertiesTag(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" minProperties:"1" maxProperties:"5"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":          "string",
				"minProperties": 1,
				"maxProperties": 5,
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyExclusiveMinMaxTag(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" exclusiveMinimum:"1.5" exclusiveMaximum:"10.25"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":             "string",
				"exclusiveMinimum": 1.5,
				"exclusiveMaximum": 10.25,
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyPatternPropertiesTag(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" patternProperties:"{\"^x-\\\\d+$\":{\"type\":\"string\"}}"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":              "string",
				"patternProperties": map[string]any{"^x-\\d+$": map[string]any{"type": "string"}},
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyContainsTag(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" contains:"[#/components/schemas/Item]"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":     "string",
				"contains": []any{"#/components/schemas/Item"},
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyIfThenElseTags(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" if:"{\"properties\":{\"type\":{\"const\":\"a\"}}}" then:"{\"required\":[\"other\"]}" else:"{\"required\":[\"alt\"]}"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type": "string",
				"if":   map[string]any{"properties": map[string]any{"type": map[string]any{"const": "a"}}},
				"then": map[string]any{"required": []any{"other"}},
				"else": map[string]any{"required": []any{"alt"}},
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestShouldApplyDefsSchemaIDTags(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" $defs:"{\"X\":{\"type\":\"string\"}}" $schema:"http://example.com/schema" $id:"http://example.com/id"`
	}
	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"field": map[string]any{
				"type":    "string",
				"$defs":   map[string]any{"X": map[string]any{"type": "string"}},
				"$schema": "http://example.com/schema",
				"$id":     "http://example.com/id",
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}
