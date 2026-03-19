# jsonpatch

Summary
-------

The `jsonpatch` package generates and applies JSON Patch operations (RFC 6902). Use
`GeneratePatch(before, after, basePath)` to produce a list of operations, and
`ApplyPatch(original, patches)` or `ApplyPatchAndHydrate(original, updated, patches)`
to apply them. See package `doc.go` for the full contract (operations, array handling,
special types).

Try it
------

Example: compute and apply a patch between two JSON documents.

```go
package main

import (
    "encoding/json"
    "fmt"

    "github.com/fgrzl/json/jsonpatch"
)

func main() {
    src := map[string]any{"a": 1, "b": 2}
    dst := map[string]any{"a": 1, "b": 3, "c": 4}

    patch, err := jsonpatch.GeneratePatch(src, dst, "")
    if err != nil {
        panic(err)
    }

    // Apply the patch to src
    patched, err := jsonpatch.ApplyPatch(src, patch)
    if err != nil {
        panic(err)
    }

    out, _ := json.MarshalIndent(patched, "", "  ")
    fmt.Println(string(out))
}
```

Notes
-----

- Supported operations: add, remove, replace, move. Paths use JSON Pointer (RFC 6901).
- Array diffs use an LCS-based heuristic; element identity is by deep equality.
- Types implementing `json.Marshaler` or `encoding.TextMarshaler` are diffed by their marshaled form.
- See the package tests for edge cases and ambiguous array identity.

Advanced scenarios
------------------

1) Array diffs and heuristics

By default the generator uses a longest-common-subsequence heuristic to compute
array edits. This works well when elements are stable or comparable. When array
elements are complex objects without stable identity, consider:

- Providing custom comparators (if your codepath allows) before generating
    patches.
- Converting arrays into maps keyed by an identity property when identity is
    important.

2) Hydration and ApplyPatchAndHydrate

If you need to apply patches directly to strongly-typed Go values, use
`ApplyPatchAndHydrate` which applies operations and attempts to unmarshal the
result back into the provided type.

This is especially useful for types whose JSON form differs from their in-memory
representation, such as `uuid.UUID`, `time.Time`, `netip.Addr`, and
`json.RawMessage`. These values are normalized through their marshaled form, so
patches are generated at the field level instead of diffing internal bytes or
unexported struct fields.

Example:

```go
package main

import (
    "fmt"
    "time"

    "github.com/google/uuid"

    "github.com/fgrzl/json/jsonpatch"
)

type Document struct {
    ID        uuid.UUID `json:"id"`
    UpdatedAt time.Time `json:"updatedAt"`
}

func main() {
    before := Document{
        ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
        UpdatedAt: time.Date(2024, time.January, 15, 10, 30, 0, 0, time.UTC),
    }
    after := Document{
        ID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
        UpdatedAt: time.Date(2024, time.January, 16, 12, 45, 0, 0, time.UTC),
    }

    patch, err := jsonpatch.GeneratePatch(before, after, "")
    if err != nil {
        panic(err)
    }

    var updated Document
    if err := jsonpatch.ApplyPatchAndHydrate(before, &updated, patch); err != nil {
        panic(err)
    }

    fmt.Println(updated.ID)
    fmt.Println(updated.UpdatedAt.Format(time.RFC3339))
}
```

3) Performance tips

- For large documents, marshal to []byte once and use streaming or chunked
    approaches around the diff generation.
- Avoid generating patches for frequently-changing large arrays; consider
    replacing entire arrays with a single `replace` op when that is cheaper.

4) Error handling

Patch application may fail when paths don't exist, types mismatch, or operations
are invalid. Always check and return errors from `ApplyPatch`/`ApplyPatchAndHydrate`.

5) Testing

- Exercise array edge-cases in unit tests (insertions, deletions, moves).
- Use `patch_test.go` as a reference for expected behaviors and failure modes.
