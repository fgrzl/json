package jsonschema

import (
	"encoding/json"
	"reflect"
	"testing"

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
