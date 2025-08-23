# jsonschema

Summary
-------

Documentation for the `jsonschema` package. This package helps generate JSON Schema for
Go types and optionally collect reusable schema components.

Try it
------

Generate a schema for a simple struct and inspect components:

```go
package main

import (
    "encoding/json"
    "fmt"

    "github.com/fgrzl/json/jsonschema"
)

func main() {
    b := jsonschema.NewBuilder()
    // Pass a value of the type you want to generate a schema for.
    s := b.Schema(struct{ Name string `json:"name"` }{})
    comps := b.Components()

    sOut, _ := json.MarshalIndent(s, "", "  ")
    compsOut, _ := json.MarshalIndent(comps, "", "  ")
    fmt.Println(string(sOut))
    fmt.Println(string(compsOut))
}
```

Notes
-----

- `Builder.Schema()` returns a schema for the provided type but does not automatically
  populate the builder components; use `SchemaWithComponents()` when you need a root
  schema that includes references to collected components.
- The `Builder` is not safe for concurrent use unless explicitly documented otherwise.

Advanced scenarios
------------------

1) Generating components and references

Use `SchemaWithComponents()` when you want the builder to collect reusable
components and produce a root schema that references them. This is useful for
large schemas with repeated types.

```go
b := jsonschema.NewBuilder()
root := b.SchemaWithComponents(myStructType)
components := b.Components()
// root may contain $ref entries pointing into components
```

2) Self-referential and recursive types

The builder attempts to detect self-references and emits `$ref` to components to
avoid infinite recursion. For recursive types (e.g., trees or linked lists), prefer
`SchemaWithComponents()` so the recursion is represented by references.

3) Nullable / SQL null types

The generator maps common SQL null types (e.g., `sql.NullString`) to a schema that
allows `null` where appropriate. If you have custom nullable wrappers, provide a
value of the underlying type or register a custom mapping.

4) Tag-driven constraints and metadata

Use struct tags to add constraints and metadata:

```go
type Person struct {
  Name  string `json:"name" minLength:"1"`
  Email string `json:"email" format:"email"`
}
```

Supported tags include numeric bounds (`minimum`, `maximum`), string lengths
(`minLength`, `maxLength`), regex `pattern`, array constraints (`minItems`,
`uniqueItems`), and custom metadata keywords like `dataSource` and `componentId`.

6) json.RawMessage and additionalProperties

The generator treats `json.RawMessage` as "raw JSON" by default. That means
the field's schema is the empty schema (no `type` or properties) unless tags
are used to further constrain it. This is helpful when the field may contain
arbitrary JSON that you don't want to model strictly in Go.

When using the `additionalProperties` struct tag you can control how the
generator represents additional properties for object-like fields:

- `additionalProperties:"false"` -> sets `additionalProperties: false` on the
  field's schema.
- `additionalProperties:"true"` -> for raw JSON fields or empty schemas the
  generator will coerce the field to an object and set `additionalProperties`
  to an empty schema (allow anything). For non-raw typed fields the field's
  existing type is preserved and `additionalProperties` is set to an empty
  schema.
- `additionalProperties:"#/definitions/MySchema"` (or any value starting
  with `#`) -> the generator sets `additionalProperties: { "$ref": "..." }`.

This allows mixing flexible raw JSON with more strictly typed nested schemas.

5) Tips & gotchas

- The builder expects a non-nil reflect.Type for the root; passing a nil value
  will panic.
- The `Components()` map is the builder's internal storage; copy it if you need
  an immutable snapshot.
- Consider using `SchemaWithComponents()` for public APIs so consumers see
  references rather than duplicated inline schemas.
