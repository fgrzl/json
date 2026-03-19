[![ci](https://github.com/fgrzl/json/actions/workflows/ci.yml/badge.svg)](https://github.com/fgrzl/json/actions/workflows/ci.yml)
[![Dependabot Updates](https://github.com/fgrzl/json/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/fgrzl/json/actions/workflows/dependabot/dependabot-updates)

# JSON

This repository contains small, focused Go packages for common JSON tasks:

- **jsonschema** — generate JSON Schema from Go types and validate JSON-like data against those schemas (draft 2019-09).
- **jsonpatch** — compute and apply RFC 6902 JSON Patch operations (add, remove, replace, move).
- **polymorphic** — register and marshal/unmarshal polymorphic types using a discriminator envelope.

## Quick start

These short examples help someone new to the project get started quickly. For more advanced usage and examples, see `docs/jsonschema.md`, `docs/jsonpatch.md`, and `docs/polymorphic.md`.

## Docs

See the `docs/` directory for package-specific guides and advanced examples:

- **docs/jsonschema.md** — schema generation and validation, tag-driven features (`json:",inline"`, `x-*`, `const`, `examples`, `$defs`), components, and nullable types.
- **docs/jsonpatch.md** — RFC 6902 patch generation and application, array heuristics, and `ApplyPatchAndHydrate` for typed values.
- **docs/polymorphic.md** — discriminator envelope, registry, and testing with `ClearRegistry`.

## Prerequisites

Go 1.20 or later (1.25 used in CI). Add the module to your project:

```bash
go get github.com/fgrzl/json
```

## jsonschema — quick start

Generate a simple schema for a Go struct:

```go
package main

import (
  "encoding/json"
  "fmt"
  "github.com/fgrzl/json/jsonschema"
)

func main() {
  b := jsonschema.NewBuilder()
  s := b.Schema(struct{ Name string `json:"name"` }{})
  out, _ := json.MarshalIndent(s, "", "  ")
  fmt.Println(string(out))
}
```

Use `Validate(schema, data)` to check decoded JSON (maps, slices, primitives) against a generated schema. See `docs/jsonschema.md` for validation, components, self-referential types, nullable fields, and tags.

## jsonpatch — quick start

Compute a patch and apply it:

```go
package main

import (
  "encoding/json"
  "fmt"
  "github.com/fgrzl/json/jsonpatch"
)

func main() {
  before := map[string]any{"a": 1, "b": 2}
  after := map[string]any{"a": 1, "b": 3, "c": 4}

  patch, err := jsonpatch.GeneratePatch(before, after, "")
  if err != nil {
    panic(err)
  }
  updated, err := jsonpatch.ApplyPatch(before, patch)
  if err != nil {
    panic(err)
  }
  out, _ := json.MarshalIndent(updated, "", "  ")
  fmt.Println(string(out))
}
```

See `docs/jsonpatch.md` for advanced scenarios: array heuristics, patch hydration, and handling for values like `uuid.UUID` and `time.Time` that marshal differently from their internal Go representation.

## polymorphic — quick start

Register a concrete type and (un)marshal via the envelope:

```go
package main

import (
  "encoding/json"
  "fmt"
  "github.com/fgrzl/json/polymorphic"
)

type Person struct{ Name string `json:"name"` }

func (p *Person) GetDiscriminator() string { return "person" }

func init() { polymorphic.RegisterType[Person]() }

func main() {
  env := polymorphic.NewEnvelope("person", &Person{Name: "Alice"})
  data, _ := polymorphic.MarshalPolymorphicJSON(env)

  inst, err := polymorphic.UnmarshalPolymorphicJSON(data)
  if err != nil { panic(err) }
  person := inst.(*Person)
  fmt.Println(person.Name)
}
```

See `docs/polymorphic.md` for advanced scenarios: custom discriminators, registry management, and testing patterns.

## Contributing and docs

Add more guides under `docs/` using the naming convention `docs/my-doc.md` (all lower-case, hyphen-separated). Each package should keep a `docs/{package}.md` file with advanced examples and edge cases.

## Running tests

Run all package tests:

```shell
go test ./...
```

Run tests with coverage and print a summary:

```shell
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Format code:

```shell
gofmt -w .
```
