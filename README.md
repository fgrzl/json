[![ci](https://github.com/fgrzl/json/actions/workflows/ci.yaml/badge.svg)](https://github.com/fgrzl/json/actions/workflows/ci.yaml)
[![ci](https://github.com/fgrzl/json/actions/workflows/pre-release.yaml/badge.svg)](https://github.com/fgrzl/json/actions/workflows/pre-release.yaml)
[![Dependabot Updates](https://github.com/fgrzl/json/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/fgrzl/json/actions/workflows/dependabot/dependabot-updates)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=fgrzl_json&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=fgrzl_json)

# JSON

This repository contains small, focused Go packages for common JSON tasks:

- `jsonschema` — generate JSON Schema from Go types.
- `jsonpatch` — compute and apply RFC 6902 JSON Patch operations.
- `polymorphic` — register and marshal/unmarshal polymorphic types using a discriminator envelope.

## Quick start

These short examples help someone new to the project get started quickly. For more advanced usage and examples, see `docs/jsonschema.md`, `docs/jsonpatch.md`, and `docs/polymorphic.md`.

## Prerequisites

Make sure you have a Go toolchain installed (Go 1.20+ recommended). Add the module to your project with the module path `github.com/fgrzl/json`.

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

See `docs/jsonschema.md` for advanced scenarios: components, self-referential types, nullable fields, and tags.

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

  patch := jsonpatch.GeneratePatch(before, after)
  updated, err := jsonpatch.ApplyPatch(before, patch)
  if err != nil {
    panic(err)
  }
  out, _ := json.MarshalIndent(updated, "", "  ")
  fmt.Println(string(out))
}
```

See `docs/jsonpatch.md` for advanced scenarios: array heuristics, patch hydration, and performance tips.

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

Run package tests locally:

```shell
go test ./...
```

For formatting:

```shell
gofmt -w .
```
