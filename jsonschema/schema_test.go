package jsonschema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertSchema(t *testing.T, input any, expected map[string]any) {
	t.Helper()
	typ := reflect.TypeOf(input)
	got := GenerateSchema(typ)
	assert.Equal(t, expected, got)
}

func assertSchemas(t *testing.T, input any, expected map[string]any) {
	t.Helper()
	typ := reflect.TypeOf(input)
	got := GenerateSchemaWithComponents(typ)
	assert.Equal(t, expected, got)
}

func TestGenerateSchema_Required(t *testing.T) {
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

func TestGenerateSchema_NumericConstraints(t *testing.T) {
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

func TestGenerateSchema_StringConstraints(t *testing.T) {
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

func TestGenerateSchema_Enum(t *testing.T) {
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

func TestGenerateSchema_AdditionalProperties(t *testing.T) {
	type Nested struct {
		Field string `json:"field"`
	}
	type TestStruct struct {
		Data Nested `json:"data" additionalProperties:"false"`
	}

	expected := map[string]any{
		"type": "object",
		"properties": map[string]any{
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

func TestGenerateSchemaWithComponents_AdditionalProperties(t *testing.T) {
	type Nested struct {
		Field string `json:"field"`
	}
	type TestStruct struct {
		Data Nested `json:"data" additionalProperties:"false"`
	}

	expected := map[string]any{
		"TestStruct": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"data": map[string]any{
					"$ref": "#/components/schemas/Nested",
				},
			},
		},
		"Nested": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"field": map[string]any{
					"type": "string",
				},
			},
			"additionalProperties": false,
		},
	}

	assertSchemas(t, TestStruct{}, expected)
}

func TestGenerateSchema_RawMessage(t *testing.T) {
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
