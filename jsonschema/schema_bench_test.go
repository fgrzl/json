package jsonschema

import (
	"reflect"
	"testing"
)

// ---------- bench types ----------

type benchSimple struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Active bool    `json:"active"`
	Score  float64 `json:"score"`
	A      string  `json:"a"`
	B      string  `json:"b"`
	C      int     `json:"c"`
	D      bool    `json:"d"`
	E      string  `json:"e"`
}

type benchNestedItem struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type benchNestedStruct struct {
	ID    string            `json:"id"`
	Items []benchNestedItem `json:"items"`
	Meta  benchNestedItem   `json:"meta"`
}

type benchLevel2 struct {
	Value string `json:"value"`
}

type benchLevel1 struct {
	Level2 benchLevel2 `json:"level2"`
}

type benchDeepNested struct {
	Level1 benchLevel1 `json:"level1"`
}

// ---------- GenerateSchema benchmarks ----------

func BenchmarkGenerateSchema_SimpleStruct(b *testing.B) {
	typ := reflect.TypeOf(benchSimple{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateSchema(typ)
	}
}

func BenchmarkGenerateSchema_NestedStruct(b *testing.B) {
	typ := reflect.TypeOf(benchNestedStruct{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateSchema(typ)
	}
}

func BenchmarkGenerateSchemaWithComponents_DeepNested(b *testing.B) {
	typ := reflect.TypeOf(benchDeepNested{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GenerateSchemaWithComponents(typ)
	}
}

func BenchmarkBuilder_Schema(b *testing.B) {
	builder := NewBuilder()
	typ := reflect.TypeOf(benchSimple{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.Schema(typ)
	}
}

func BenchmarkSchemaFrom_T(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SchemaFrom[benchSimple]()
	}
}

// ---------- Validate benchmarks ----------

func BenchmarkValidate_SimpleStruct(b *testing.B) {
	schema := GenerateSchema(reflect.TypeOf(benchSimple{}))
	data := map[string]any{
		"id": 1.0, "name": "a", "email": "b", "active": true, "score": 1.5,
		"a": "x", "b": "y", "c": 0, "d": false, "e": "z",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(schema, data)
	}
}

func BenchmarkValidate_Nested(b *testing.B) {
	schema := GenerateSchema(reflect.TypeOf(benchNestedStruct{}))
	data := map[string]any{
		"id": "x",
		"items": []any{
			map[string]any{"label": "a", "value": 1.0},
		},
		"meta": map[string]any{"label": "b", "value": 2.0},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(schema, data)
	}
}

func BenchmarkValidate_Invalid(b *testing.B) {
	schema := GenerateSchema(reflect.TypeOf(benchSimple{}))
	data := map[string]any{"id": "not a number", "name": 123}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(schema, data)
	}
}
