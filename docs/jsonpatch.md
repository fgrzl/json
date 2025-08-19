# jsonpatch

Summary
-------

Documentation for the `jsonpatch` package. This package provides utilities to generate and
apply JSON Patch (RFC 6902) operations between two JSON documents.

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

    patch := jsonpatch.GeneratePatch(src, dst)
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

- The patch generation uses a best-effort heuristic for arrays (LCS-based).
- See the package tests for edge cases and behavior when array element identity is ambiguous.

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
