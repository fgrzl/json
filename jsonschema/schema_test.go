package jsonschema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertSchema(t *testing.T, input interface{}, expected map[string]interface{}) {
	t.Helper()
	typ := reflect.TypeOf(input)
	got := GenerateSchema(typ)
	assert.Equal(t, expected, got)
}

func TestGenerateSchema_Required(t *testing.T) {
	type TestStruct struct {
		ID   int    `json:"id" required:"true"`
		Name string `json:"name"`
		Age  int    `json:"age" required:"true"`
	}
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":   map[string]interface{}{"type": "integer"},
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "integer"},
		},
		"required": []string{"id", "age"},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestGenerateSchema_NumericConstraints(t *testing.T) {
	type TestStruct struct {
		Number int `json:"number" minimum:"0" maximum:"100" multipleOf:"2"`
	}
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"number": map[string]interface{}{
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
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
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
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"choice": map[string]interface{}{
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
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field": map[string]interface{}{
						"type": "string",
					},
				},
				"additionalProperties": false,
			},
		},
	}
	assertSchema(t, TestStruct{}, expected)
}

func TestGenerateSchema_RawMessage(t *testing.T) {
	type TestStruct struct {
		ID   int    `json:"id" required:"true"`
		Name string `json:"name"`
		Age  int    `json:"age" required:"true"`
	}
	typ := reflect.TypeOf(TestStruct{})
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":   map[string]interface{}{"type": "integer"},
			"name": map[string]interface{}{"type": "string"},
			"age":  map[string]interface{}{"type": "integer"},
		},
		"required": []string{"id", "age"},
	}
	expectedBytes, _ := json.Marshal(expected)
	got := GenerateSchemaRawMessage(typ)
	assert.JSONEq(t, string(expectedBytes), string(got))
}
