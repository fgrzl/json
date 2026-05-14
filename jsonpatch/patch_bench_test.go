package jsonpatch

import (
	"encoding/json"
	"strconv"
	"testing"
)

// ---------- helpers ----------

func flatDoc(n int) map[string]any {
	doc := make(map[string]any, n)
	for i := 0; i < n; i++ {
		doc["key"+strconv.Itoa(i)] = i
	}
	return doc
}

func nestedDoc(depth int) map[string]any {
	if depth == 0 {
		return map[string]any{"leaf": "value"}
	}
	return map[string]any{"child": nestedDoc(depth - 1)}
}

func arrayDoc(n int) map[string]any {
	arr := make([]any, n)
	for i := 0; i < n; i++ {
		arr[i] = map[string]any{"id": i, "name": "item" + strconv.Itoa(i)}
	}
	return map[string]any{"items": arr}
}

type benchStruct struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Email   string `json:"email"`
	Active  bool   `json:"active"`
	Score   int    `json:"score,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type benchNestedStruct struct {
	ID      string      `json:"id"`
	Profile benchStruct `json:"profile"`
	Tags    []string    `json:"tags"`
}

// ---------- GeneratePatch benchmarks ----------

func BenchmarkGeneratePatch_FlatMap_10Keys(b *testing.B) {
	before := flatDoc(10)
	after := flatDoc(10)
	after["key5"] = "changed"
	after["newKey"] = "added"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_FlatMap_100Keys(b *testing.B) {
	before := flatDoc(100)
	after := flatDoc(100)
	after["key50"] = "changed"
	after["key99"] = "changed"
	delete(after, "key0")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_FlatMap_1000Keys(b *testing.B) {
	before := flatDoc(1000)
	after := flatDoc(1000)
	for j := 0; j < 50; j++ {
		after["key"+strconv.Itoa(j*20)] = "changed"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_Nested_Depth5(b *testing.B) {
	before := nestedDoc(5)
	after := nestedDoc(5)
	inner := after["child"].(map[string]any)["child"].(map[string]any)["child"].(map[string]any)["child"].(map[string]any)["child"].(map[string]any)
	inner["leaf"] = "changed"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_Nested_Depth10(b *testing.B) {
	before := nestedDoc(10)
	after := nestedDoc(10)
	cur := after
	for d := 0; d < 10; d++ {
		cur = cur["child"].(map[string]any)
	}
	cur["leaf"] = "changed"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_Array_10Elements(b *testing.B) {
	before := arrayDoc(10)
	after := arrayDoc(10)
	after["items"].([]any)[5].(map[string]any)["name"] = "changed"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_Array_100Elements(b *testing.B) {
	before := arrayDoc(100)
	after := arrayDoc(100)
	after["items"].([]any)[50].(map[string]any)["name"] = "changed"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_Array_1000Elements(b *testing.B) {
	before := arrayDoc(1000)
	after := arrayDoc(1000)
	after["items"].([]any)[500].(map[string]any)["name"] = "changed"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_Struct(b *testing.B) {
	before := benchStruct{Name: "Alice", Age: 30, Email: "alice@example.com", Active: true}
	after := benchStruct{Name: "Alice", Age: 31, Email: "alice@new.com", Active: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_NestedStruct(b *testing.B) {
	before := benchNestedStruct{
		ID:      "1",
		Profile: benchStruct{Name: "Alice", Age: 30, Email: "a@b.com", Active: true},
		Tags:    []string{"go", "dev"},
	}
	after := benchNestedStruct{
		ID:      "1",
		Profile: benchStruct{Name: "Alice", Age: 31, Email: "a@new.com", Active: false},
		Tags:    []string{"go", "dev", "senior"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(before, after, "")
	}
}

func BenchmarkGeneratePatch_NoDiff(b *testing.B) {
	doc := flatDoc(50)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GeneratePatch(doc, doc, "")
	}
}

// ---------- ApplyPatch benchmarks ----------

func BenchmarkApplyPatch_SingleAdd(b *testing.B) {
	doc := flatDoc(10)
	patches := []Patch{{Op: "add", Path: "/newKey", Value: "newVal"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_SingleReplace(b *testing.B) {
	doc := flatDoc(10)
	patches := []Patch{{Op: "replace", Path: "/key5", Value: "replaced"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_SingleRemove(b *testing.B) {
	doc := flatDoc(10)
	patches := []Patch{{Op: "remove", Path: "/key5"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_SingleMove(b *testing.B) {
	doc := flatDoc(10)
	patches := []Patch{{Op: "move", From: "/key0", Path: "/moved"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_SingleCopy(b *testing.B) {
	doc := flatDoc(10)
	patches := []Patch{{Op: "copy", From: "/key0", Path: "/copied"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_SingleTest(b *testing.B) {
	doc := flatDoc(10)
	patches := []Patch{{Op: "test", Path: "/key5", Value: 5}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_BatchOps_10(b *testing.B) {
	doc := flatDoc(20)
	patches := make([]Patch, 10)
	for i := 0; i < 10; i++ {
		patches[i] = Patch{Op: "replace", Path: "/key" + strconv.Itoa(i), Value: "new" + strconv.Itoa(i)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_BatchOps_100(b *testing.B) {
	doc := flatDoc(200)
	patches := make([]Patch, 100)
	for i := 0; i < 100; i++ {
		patches[i] = Patch{Op: "replace", Path: "/key" + strconv.Itoa(i), Value: "new" + strconv.Itoa(i)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_DeepPath(b *testing.B) {
	doc := nestedDoc(10)
	patches := []Patch{{Op: "replace", Path: "/child/child/child/child/child/child/child/child/child/child/leaf", Value: "new"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_ArrayAppendDash(b *testing.B) {
	doc := arrayDoc(100)
	patches := []Patch{{Op: "add", Path: "/items/-", Value: map[string]any{"id": 100, "name": "appended"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_ArrayInsertMiddle(b *testing.B) {
	doc := arrayDoc(100)
	patches := []Patch{{Op: "add", Path: "/items/50", Value: map[string]any{"id": 999, "name": "inserted"}}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

func BenchmarkApplyPatch_ArrayRemoveMiddle(b *testing.B) {
	doc := arrayDoc(100)
	patches := []Patch{{Op: "remove", Path: "/items/50"}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyPatch(doc, patches)
	}
}

// ---------- Roundtrip benchmarks (Generate + Apply) ----------

func BenchmarkRoundtrip_FlatMap_50Keys(b *testing.B) {
	before := flatDoc(50)
	after := flatDoc(50)
	after["key25"] = "changed"
	after["key49"] = "changed"
	delete(after, "key0")
	after["brand_new"] = "added"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		patches, _ := GeneratePatch(before, after, "")
		_, _ = ApplyPatch(before, patches)
	}
}

func BenchmarkRoundtrip_NestedStruct(b *testing.B) {
	before := benchNestedStruct{
		ID:      "1",
		Profile: benchStruct{Name: "Alice", Age: 30, Email: "a@b.com", Active: true},
		Tags:    []string{"go", "dev"},
	}
	after := benchNestedStruct{
		ID:      "1",
		Profile: benchStruct{Name: "Alice", Age: 31, Email: "a@new.com", Active: false},
		Tags:    []string{"go", "dev", "senior"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		patches, _ := GeneratePatch(before, after, "")
		_, _ = ApplyPatch(before, patches)
	}
}

func BenchmarkRoundtrip_Array_100Elements(b *testing.B) {
	before := arrayDoc(100)
	after := arrayDoc(100)
	after["items"].([]any)[50].(map[string]any)["name"] = "changed"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		patches, _ := GeneratePatch(before, after, "")
		_, _ = ApplyPatch(before, patches)
	}
}

// ---------- JSON Marshal/Unmarshal benchmarks ----------

func BenchmarkPatchMarshalJSON(b *testing.B) {
	patches := []Patch{
		{Op: "add", Path: "/foo", Value: "bar"},
		{Op: "remove", Path: "/baz"},
		{Op: "replace", Path: "/qux", Value: 42},
		{Op: "move", From: "/a", Path: "/b"},
		{Op: "copy", From: "/c", Path: "/d"},
		{Op: "test", Path: "/e", Value: true},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(patches)
	}
}

func BenchmarkPatchUnmarshalJSON(b *testing.B) {
	data := []byte(`[{"op":"add","path":"/foo","value":"bar"},{"op":"remove","path":"/baz","value":null},{"op":"replace","path":"/qux","value":42},{"op":"move","from":"/a","path":"/b","value":null},{"op":"copy","from":"/c","path":"/d","value":null},{"op":"test","path":"/e","value":true}]`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var patches []Patch
		_ = json.Unmarshal(data, &patches)
	}
}

// ---------- DeepCopy benchmarks ----------

func BenchmarkDeepCopy_FlatMap_100Keys(b *testing.B) {
	doc := flatDoc(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepCopy(doc)
	}
}

func BenchmarkDeepCopy_Nested_Depth10(b *testing.B) {
	doc := nestedDoc(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepCopy(doc)
	}
}

func BenchmarkDeepCopy_Array_100Elements(b *testing.B) {
	doc := arrayDoc(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deepCopy(doc)
	}
}

// ---------- toMap (struct conversion) benchmarks ----------

func BenchmarkToMap_SimpleStruct(b *testing.B) {
	s := benchStruct{Name: "Alice", Age: 30, Email: "a@b.com", Active: true, Score: 100}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = toMap(s)
	}
}

func BenchmarkToMap_NestedStruct(b *testing.B) {
	s := benchNestedStruct{
		ID:      "1",
		Profile: benchStruct{Name: "Alice", Age: 30, Email: "a@b.com", Active: true},
		Tags:    []string{"go", "dev"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = toMap(s)
	}
}

func BenchmarkToMap_EmbeddedStruct(b *testing.B) {
	type Base struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	type Extended struct {
		Base

		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	s := Extended{Base: Base{ID: "1", Type: "test"}, Name: "Alice", Value: 42}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = toMap(s)
	}
}
