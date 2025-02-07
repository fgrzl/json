[![ci](https://github.com/fgrzl/json/actions/workflows/ci.yml/badge.svg)](https://github.com/fgrzl/json/actions/workflows/ci.yml)
# JSON 

## JSON Schema

This package provides functionality to generate JSON schemas from Go types.

## Functions

### GenerateSchema

The `GenerateSchema` function takes a `reflect.Type` and generates a JSON schema as a `map[string]interface{}`. It supports the following Go types:

- `struct`: Generates an "object" type schema with properties.
- `slice` and `array`: Generates an "array" type schema with items.
- `map`: Generates an "object" type schema with additional properties.
- `int`, `int8`, `int16`, `int32`, `int64`: Generates an "integer" type schema.
- `float32`, `float64`: Generates a "number" type schema.
- `bool`: Generates a "boolean" type schema.
- `string`: Generates a "string" type schema.
- Other types default to a "string" type schema.

```go
func GenerateSchema(t reflect.Type) map[string]interface{}
```

Example usage:

```go
package main

import (
    "fmt"
    "reflect"
    "jsonschema"
)

type Example struct {
    Name        string `json:"name" title:"Name" description:"The name of the person" minLength:"1" maxLength:"100"`
    Age         int    `json:"age" minimum:"0" maximum:"150"`
    Email       string `json:"email" format:"email"`
    Tags        []string `json:"tags" minItems:"1" uniqueItems:"true"`
    Preferences map[string]interface{} `json:"preferences" additionalProperties:"true"`
    Password    string `json:"password" required:"true" pattern:"^[a-zA-Z0-9]{8,}$"`
}

func main() {
    schema := jsonschema.GenerateSchema(reflect.TypeOf(Example{}))
    fmt.Println(schema)
}
```

### Supported Tags

- **Numeric constraints**:
  - `minimum`: Specifies the minimum value.
  - `maximum`: Specifies the maximum value.
  - `multipleOf`: Specifies that the value must be a multiple of this number.

- **String constraints**:
  - `minLength`: Specifies the minimum length of the string.
  - `maxLength`: Specifies the maximum length of the string.
  - `pattern`: Specifies a regular expression that the string must match.
  - `format`: Specifies the format of the string (e.g., `email`, `date`).

- **Array constraints**:
  - `minItems`: Specifies the minimum number of items in the array.
  - `maxItems`: Specifies the maximum number of items in the array.
  - `uniqueItems`: Specifies whether all items in the array must be unique.

- **Enum support**:
  - `enum`: Specifies a comma-separated list of valid values.

- **Metadata**:
  - `title`: Specifies the title of the schema.
  - `description`: Specifies the description of the schema.
  - `default`: Specifies the default value.

- **Composition keywords**:
  - `oneOf`: Specifies a comma-separated list of schemas, one of which must be valid.
  - `anyOf`: Specifies a comma-separated list of schemas, any of which must be valid.
  - `allOf`: Specifies a comma-separated list of schemas, all of which must be valid.
  - `not`: Specifies a schema that must not be valid.

- **Required field**:
  - `required`: Specifies whether the field is required.

- **Additional properties**:
  - `additionalProperties`: Specifies whether additional properties are allowed (e.g., `true`, `false`, or a schema reference).

## JSON Patch

The `jsonpatch` package provides utilities to generate and apply JSON Patch operations. JSON Patch is a format for describing changes to a JSON document. It can be used to update a JSON document by sending the changes rather than the entire document.

### Features

- Generate JSON Patch operations by comparing two JSON documents.
- Apply JSON Patch operations to a JSON document.
- Supports add, remove, replace, and move operations.

### Usage

To generate a patch:

```go
import "github.com/fgrzl/json/jsonpatch"

before := map[string]interface{}{"foo": "bar"}
after := map[string]interface{}{"foo": "baz"}

patches, err := jsonpatch.GeneratePatch(before, after, "")
if err != nil {
    // handle error
}
```

To apply a patch:

```go
import "github.com/fgrzl/json/jsonpatch"

original := map[string]interface{}{"foo": "bar"}
patches := []jsonpatch.Patch{
    {Op: "replace", Path: "/foo", Value: "baz"},
}

updated, err := jsonpatch.ApplyPatch(original, patches)
if err != nil {
    // handle error
}
```
