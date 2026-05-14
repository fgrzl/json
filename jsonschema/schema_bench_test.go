package jsonschema

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
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

type benchHotPathStruct struct {
	ExpiresAt    time.Time  `json:"expires_at" description:"When the client credential expires"`
	RevokedAt    *time.Time `json:"revoked_at" description:"When the client was revoked"`
	CredentialID uuid.UUID  `json:"credential_id" x-component-id:"secret-picker" title:"Secret" description:"UUID of the stored credential used to authenticate to the provider"`
	Status       string     `json:"status" enum:"active,revoked"`
}

func benchmarkSetup(b *testing.B) {
	b.Helper()
	b.ReportAllocs()
}

// ---------- GenerateSchema benchmarks ----------

func BenchmarkGenerateSchema_SimpleStruct(b *testing.B) {
	benchmarkSetup(b)
	typ := reflect.TypeOf(benchSimple{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateSchema(typ)
	}
}

func BenchmarkGenerateSchema_NestedStruct(b *testing.B) {
	benchmarkSetup(b)
	typ := reflect.TypeOf(benchNestedStruct{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateSchema(typ)
	}
}

func BenchmarkGenerateSchemaWithComponents_DeepNested(b *testing.B) {
	benchmarkSetup(b)
	typ := reflect.TypeOf(benchDeepNested{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GenerateSchemaWithComponents(typ)
	}
}

func BenchmarkBuilder_Schema(b *testing.B) {
	benchmarkSetup(b)
	builder := NewBuilder()
	typ := reflect.TypeOf(benchSimple{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.Schema(typ)
	}
}

func BenchmarkSchemaFrom_T(b *testing.B) {
	benchmarkSetup(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SchemaFrom[benchSimple]()
	}
}

func BenchmarkGenerateSchemaRawMessage_HotCache(b *testing.B) {
	benchmarkSetup(b)
	typ := reflect.TypeOf(benchHotPathStruct{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateSchemaRawMessage(typ)
	}
}

func BenchmarkGenerateSchema_TaggedHotPath(b *testing.B) {
	benchmarkSetup(b)
	typ := reflect.TypeOf(benchHotPathStruct{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateSchema(typ)
	}
}

// ---------- Validate benchmarks ----------

func BenchmarkValidate_SimpleStruct(b *testing.B) {
	benchmarkSetup(b)
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
	benchmarkSetup(b)
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
	benchmarkSetup(b)
	schema := GenerateSchema(reflect.TypeOf(benchSimple{}))
	data := map[string]any{"id": "not a number", "name": 123}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(schema, data)
	}
}

func BenchmarkValidate_TaggedHotPath(b *testing.B) {
	benchmarkSetup(b)
	schema := GenerateSchema(reflect.TypeOf(benchHotPathStruct{}))
	data := map[string]any{
		"expires_at":    time.Date(2026, time.May, 14, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"revoked_at":    time.Date(2026, time.May, 15, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"credential_id": uuid.MustParse("123e4567-e89b-12d3-a456-426614174000").String(),
		"status":        "active",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(schema, data)
	}
}

func BenchmarkValidate_PatternPropertiesHeavy(b *testing.B) {
	benchmarkSetup(b)
	schema := map[string]any{
		TypeKey: TypeObject,
		PatternPropertiesKey: map[string]any{
			`^x-.*-0$`: map[string]any{TypeKey: TypeString},
			`^x-.*-1$`: map[string]any{TypeKey: TypeInteger},
			`^x-.*-2$`: map[string]any{TypeKey: TypeBoolean},
			`^x-.*-3$`: map[string]any{TypeKey: TypeString},
			`^x-.*-4$`: map[string]any{TypeKey: TypeInteger},
			`^x-.*-5$`: map[string]any{TypeKey: TypeBoolean},
			`^x-.*-6$`: map[string]any{TypeKey: TypeString},
			`^x-.*-7$`: map[string]any{TypeKey: TypeInteger},
			`^x-.*-8$`: map[string]any{TypeKey: TypeBoolean},
			`^x-.*-9$`: map[string]any{TypeKey: TypeString},
		},
	}
	data := map[string]any{
		"x-alpha-0":   "a",
		"x-beta-1":    1.0,
		"x-gamma-2":   true,
		"x-delta-3":   "d",
		"x-epsilon-4": 4.0,
		"x-zeta-5":    false,
		"x-eta-6":     "g",
		"x-theta-7":   7.0,
		"x-iota-8":    true,
		"x-kappa-9":   "j",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Validate(schema, data)
	}
}
