package jsonschema

import (
	"reflect"
	"testing"
)

func TestGenerateSchema_Required(t *testing.T) {
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
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Required test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_NumericConstraints(t *testing.T) {
	type TestStruct struct {
		Number int `json:"number" minimum:"0" maximum:"100" multipleOf:"2"`
	}
	typ := reflect.TypeOf(TestStruct{})
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
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Numeric constraints test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_StringConstraints(t *testing.T) {
	type TestStruct struct {
		Text string `json:"text" minLength:"3" maxLength:"10" pattern:"^[a-z]+$" format:"email"`
	}
	typ := reflect.TypeOf(TestStruct{})
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
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("String constraints test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_ArrayConstraints(t *testing.T) {
	type TestStruct struct {
		Items []string `json:"items" minItems:"1" maxItems:"5" uniqueItems:"true"`
	}
	typ := reflect.TypeOf(TestStruct{})
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"minItems":    1,
				"maxItems":    5,
				"uniqueItems": true,
			},
		},
	}
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Array constraints test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_Enum(t *testing.T) {
	type TestStruct struct {
		Choice string `json:"choice" enum:"option1, option2, option3"`
	}
	typ := reflect.TypeOf(TestStruct{})
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"choice": map[string]interface{}{
				"type": "string",
				"enum": []string{"option1", "option2", "option3"},
			},
		},
	}
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Enum test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_Metadata(t *testing.T) {
	type TestStruct struct {
		Field string `json:"field" title:"My Field" description:"This is a test field" default:"default value"`
	}
	typ := reflect.TypeOf(TestStruct{})
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"field": map[string]interface{}{
				"type":        "string",
				"title":       "My Field",
				"description": "This is a test field",
				"default":     "default value",
			},
		},
	}
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Metadata test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_Composition(t *testing.T) {
	type TestStruct struct {
		Composed string `json:"composed" oneOf:"#/definitions/TypeA, #/definitions/TypeB" anyOf:"#/definitions/TypeC" allOf:"#/definitions/TypeD" not:"#/definitions/TypeE"`
	}
	typ := reflect.TypeOf(TestStruct{})
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"composed": map[string]interface{}{
				"type": "string",
				"oneOf": []interface{}{
					map[string]interface{}{"$ref": "#/definitions/TypeA"},
					map[string]interface{}{"$ref": "#/definitions/TypeB"},
				},
				"anyOf": []interface{}{
					map[string]interface{}{"$ref": "#/definitions/TypeC"},
				},
				"allOf": []interface{}{
					map[string]interface{}{"$ref": "#/definitions/TypeD"},
				},
				"not": map[string]interface{}{
					"$ref": "#/definitions/TypeE",
				},
			},
		},
	}
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Composition test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_AdditionalProperties(t *testing.T) {
	type Nested struct {
		Field string `json:"field"`
	}
	type TestStruct struct {
		Data Nested `json:"data" additionalProperties:"false"`
	}
	typ := reflect.TypeOf(TestStruct{})
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
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("AdditionalProperties test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}

func TestGenerateSchema_Reference(t *testing.T) {
	type TestStruct struct {
		RefField int `json:"refField" ref:"#/definitions/CustomInt"`
	}
	typ := reflect.TypeOf(TestStruct{})
	expected := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"refField": map[string]interface{}{
				"$ref": "#/definitions/CustomInt",
			},
		},
	}
	got := GenerateSchema(typ)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Reference test failed.\nExpected: %#v\nGot: %#v", expected, got)
	}
}
