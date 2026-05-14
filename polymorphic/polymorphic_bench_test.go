package polymorphic

import (
	"encoding/json"
	"testing"
)

// benchPerson and benchCar are used by benchmarks so that benchmark code
// does not depend on test-only types. They duplicate the shape of Person/Car.
type benchPerson struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (p *benchPerson) GetDiscriminator() string { return "bench-person" }

type benchCar struct {
	Make  string `json:"make"`
	Model string `json:"model"`
}

func (c *benchCar) GetDiscriminator() string { return "bench-car" }

func initBenchRegistry(b *testing.B) {
	b.Helper()
	ClearRegistry()
	Register(func() *benchPerson { return &benchPerson{} })
	Register(func() *benchCar { return &benchCar{} })
}

// ---------- Registry benchmarks ----------

func BenchmarkCreateInstance(b *testing.B) {
	initBenchRegistry(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CreateInstance("bench-person")
	}
}

func BenchmarkLoadFactory(b *testing.B) {
	initBenchRegistry(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadFactory("bench-person")
	}
}

// ---------- Marshal/Unmarshal benchmarks ----------

func BenchmarkMarshalPolymorphicJSON(b *testing.B) {
	initBenchRegistry(b)
	obj := &benchPerson{Name: "Alice", Age: 30}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = MarshalPolymorphicJSON(obj)
	}
}

var benchEnvelopeBytes = func() []byte {
	ClearRegistry()
	Register(func() *benchPerson { return &benchPerson{} })
	b, _ := MarshalPolymorphicJSON(&benchPerson{Name: "Alice", Age: 30})
	return b
}()

func BenchmarkUnmarshalPolymorphicJSON(b *testing.B) {
	initBenchRegistry(b)
	data := benchEnvelopeBytes
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UnmarshalPolymorphicJSON(data)
	}
}

func BenchmarkRoundtrip_MarshalUnmarshal(b *testing.B) {
	initBenchRegistry(b)
	obj := &benchPerson{Name: "Alice", Age: 30}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _ := MarshalPolymorphicJSON(obj)
		_, _ = UnmarshalPolymorphicJSON(data)
	}
}

func BenchmarkEnvelope_MarshalJSON(b *testing.B) {
	initBenchRegistry(b)
	env := NewEnvelope(&benchPerson{Name: "Alice", Age: 30})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(env)
	}
}

func BenchmarkEnvelope_UnmarshalJSON(b *testing.B) {
	initBenchRegistry(b)
	data := benchEnvelopeBytes
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var e Envelope
		_ = json.Unmarshal(data, &e)
	}
}

// ---------- ToPage benchmark ----------

func benchmarkToPageN(b *testing.B, n int) {
	initBenchRegistry(b)
	envelopes := make([]*Envelope, n)
	for i := 0; i < n; i++ {
		envelopes[i] = NewEnvelope(&benchPerson{Name: "Alice", Age: 30})
	}
	page := &PolymorphicPage{Envelopes: envelopes}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToPage[*benchPerson](page)
	}
}

func BenchmarkToPage_10(b *testing.B)  { benchmarkToPageN(b, 10) }
func BenchmarkToPage_100(b *testing.B) { benchmarkToPageN(b, 100) }
