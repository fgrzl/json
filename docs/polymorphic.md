# polymorphic

Summary
-------

Documentation for the `polymorphic` package. This package provides a small registry and
helpers for marshaling/unmarshaling polymorphic types using a discriminator field (envelope).

Try it
------

Registering a type and decoding an envelope at runtime:

```go
package main

import (
    "encoding/json"
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
    // Example envelope JSON that contains a discriminator and content
    raw := []byte(`{"discriminator":"person","content":{"name":"Alice"}}`)

    inst, err := polymorphic.UnmarshalPolymorphicJSON(raw)
    if err != nil {
        panic(err)
    }

    person, ok := inst.(*Person)
    if !ok {
        panic("unexpected type")
    }
    fmt.Println(person.Name)
}
```

Notes
-----

- Use `Register`/`RegisterType` to register factories or concrete types with their discriminators.
- The registry is global; tests should call `polymorphic.ClearRegistry()` to avoid cross-test leakage.

Advanced scenarios
------------------

1) Custom discriminators and naming

By default types registered with `RegisterType[T]()` use the discriminator returned
by the `GetDiscriminator()` method on the value. If you need a different mapping
you can use `Register(discriminator, factory)` to register an explicit factory.

2) Envelope formats

The package expects an envelope with a `discriminator` field and `content` field by
default. If you have a different envelope shape, create a thin adapter that
extracts the discriminator and raw content, then call `CreateInstance`/`LoadFactory`
or `UnmarshalPolymorphicJSON` with the adapted bytes.

3) Testing best practices

- Always call `polymorphic.ClearRegistry()` in test setup/teardown to avoid
    test leakage.
- Prefer `RegisterType[T]()` inside test init functions when testing
    deserialization of concrete types.

4) Thread-safety and global state

The registry is a global map of factories. Mutating the registry concurrently is
not safe; register all types during program initialization when possible.

5) Example: custom factory and dynamic creation

```go
polymorphic.Register("custom-user", func() any { return &User{} })
inst, err := polymorphic.CreateInstance("custom-user")
// inst will be an empty *User
```

