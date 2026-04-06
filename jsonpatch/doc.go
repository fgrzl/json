// Package jsonpatch generates and applies JSON Patch operations as defined by RFC 6902.
//
// # API
//
// GeneratePatch(before, after, basePath) produces a slice of Patch operations that
// transform the before document into the after document. Both inputs may be Go structs
// or map[string]any; they are normalized to a JSON-like map representation. basePath
// is a JSON Pointer prefix (e.g. "" for the root or "/items" for a nested path).
//
// ApplyPatch(original, patches) applies the operations in order and returns the
// result as map[string]any. ApplyPatchAndHydrate(original, updated, patches) applies
// the patch and unmarshals the result into the typed updated value, which is useful
// for types whose JSON form differs from their in-memory representation (e.g.
// uuid.UUID, time.Time, json.RawMessage).
//
// ApplyPatch is object-root oriented: it always returns map[string]any. The empty
// JSON Pointer path targets the document root. Root add/replace operations require
// an object value, root test compares the entire document, and root remove/move
// operations are rejected.
//
// # Operations
//
// Supported operations: add, remove, replace, move, copy, and test. Path and From
// use JSON Pointer (RFC 6901). The implementation applies patches sequentially and
// returns an error on the first failing operation.
//
// # Array handling
//
// Patch generation uses a longest-common-subsequence (LCS) heuristic for arrays to
// produce minimal edit sequences. Element identity is based on deep equality;
// for complex arrays without stable identity, consider replacing whole arrays or
// keying by an identity field.
//
// # Special types
//
// Values that implement json.Marshaler or encoding.TextMarshaler are diffed using
// their marshaled form, so patches are expressed at the JSON level rather than
// on internal Go layout.
package jsonpatch
