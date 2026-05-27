# polymorphic

Summary
-------

The `polymorphic` package marshals and unmarshals polymorphic JSON using a discriminator
envelope: a top-level object with `$type` (discriminator) and `content` (payload).
Register types with `RegisterType[T]()` or `Register(func() *MyType { ... })`; use
`RegisterWithDiscriminator` when you need an explicit discriminator-to-factory mapping.
Use `MarshalPolymorphicJSON` / `UnmarshalPolymorphicJSON` for the wire format. See
package `doc.go` for the full contract (wire format, global registry, ClearRegistry).

Try it
------

Registering a type and decoding an envelope at runtime:

```go
package main

import (
    "fmt"

    "github.com/fgrzl/json/polymorphic"
)

type Person struct { 
    Name string `json:"name"`
}

func (p *Person) GetDiscriminator() string { return "person" }

func init() {
    polymorphic.RegisterType[Person]()
}

func main() {
    raw := []byte(`{"$type":"person","content":{"name":"Alice"}}`)

    envelope, err := polymorphic.UnmarshalPolymorphicJSON(raw)
    if err != nil {
        panic(err)
    }

    person, ok := envelope.Content.(*Person)
    if !ok {
        panic("unexpected type")
    }
    fmt.Println(person.Name)
}
```

Notes
-----

- Use `RegisterType[T]()` or `Register(func() *MyType { ... })` to register types.
- Use `RegisterWithDiscriminator` when you need an explicit discriminator string.
- The registry is process-wide global state; call `ClearRegistry()` in tests to remove custom registrations and restore package defaults.
- Registry lookups are optimized for read-heavy use, so prefer registration during initialization instead of frequent runtime churn.

Advanced scenarios
------------------

1) Custom discriminators and naming

By default types registered with `RegisterType[T]()` use the discriminator returned
by the `GetDiscriminator()` method on the value. If you need a different mapping
you can use `RegisterWithDiscriminator(discriminator, factory)` to register an explicit factory.

2) Envelope formats

The package expects an envelope with a `$type` field and `content` field by
default. If you have a different envelope shape, create a thin adapter that
extracts the discriminator and raw content, then call `CreateInstance`/`LoadFactory`
or `UnmarshalPolymorphicJSON` with the adapted bytes.

3) Testing best practices

- Always call `polymorphic.ClearRegistry()` in test setup/teardown to avoid
    test leakage.
- Prefer `RegisterType[T]()` inside test init functions when testing
    deserialization of concrete types.

4) Thread-safety and global state

The registry is process-wide global state. Concurrent registration, lookup, and
reset are synchronized, but registrations still affect the whole process. Use
`ClearRegistry()` to restore the built-in defaults in tests, and register
application types during initialization when possible.

5) Example: custom factory and dynamic creation

```go
polymorphic.RegisterWithDiscriminator("custom-user", func() any { return &User{} })
inst, err := polymorphic.CreateInstance("custom-user")
// inst will be an empty *User
```

