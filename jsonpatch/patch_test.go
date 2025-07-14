package jsonpatch

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePatch_NoChanges(t *testing.T) {
	// Arrange
	before := map[string]any{
		"name": "Alice",
		"age":  30,
	}
	after := map[string]any{
		"name": "Alice",
		"age":  30,
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 0, len(patch), "No patch should be generated when there are no changes")
}

func TestGeneratePatch_FlatChanges(t *testing.T) {
	// Arrange
	before := map[string]any{
		"name":   "Alice",
		"age":    30,
		"city":   "NYC",
		"status": "active",
	}
	after := map[string]any{
		"name":    "Alice", // unchanged
		"age":     31,      // replaced
		"country": "USA",   // added
		// "city" and "status" removed
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	// Expected ops: remove for /city and /status, replace for /age, add for /country.
	opMap := make(map[string]Patch)
	for _, op := range patch {
		opMap[op.Path] = op
	}
	// Check removal of "city"
	remOp, exists := opMap["/city"]
	assert.True(t, exists, "Expected remove op for /city")
	assert.Equal(t, "remove", remOp.Op)
	// Check removal of "status"
	remOp, exists = opMap["/status"]
	assert.True(t, exists, "Expected remove op for /status")
	assert.Equal(t, "remove", remOp.Op)
	// Check replacement of "age"
	repOp, exists := opMap["/age"]
	assert.True(t, exists, "Expected replace op for /age")
	assert.Equal(t, "replace", repOp.Op)
	assert.EqualValues(t, 31, repOp.Value)
	// Check addition of "country"
	addOp, exists := opMap["/country"]
	assert.True(t, exists, "Expected add op for /country")
	assert.Equal(t, "add", addOp.Op)
	assert.Equal(t, "USA", addOp.Value)
}

func TestGeneratePatch_NestedChanges(t *testing.T) {
	// Arrange
	before := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"email": "alice@old.com",
		},
	}
	after := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"email": "alice@new.com", // replaced
			"age":   25,              // added
		},
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	var foundReplace, foundAdd bool
	for _, op := range patch {
		if op.Path == "/user/email" && op.Op == "replace" {
			foundReplace = true
		}
		if op.Path == "/user/age" && op.Op == "add" {
			foundAdd = true
		}
	}
	assert.True(t, foundReplace, "Expected replace op for /user/email")
	assert.True(t, foundAdd, "Expected add op for /user/age")
}

func TestGeneratePatch_ArrayAdd(t *testing.T) {
	// Arrange: an element is added at the end of the array.
	before := map[string]any{
		"list": []any{1, 2, 3},
	}
	after := map[string]any{
		"list": []any{1, 2, 3, 4},
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	found := false
	for _, op := range patch {
		if op.Path == "/list/3" && op.Op == "add" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected add op for new array element at index 3")
}

func TestGeneratePatch_ArrayReplace(t *testing.T) {
	// Arrange
	before := map[string]any{
		"name": "Alice",
		"details": map[string]any{
			"age":  31,
			"city": "NYC",
		},
		"hobbies": []any{"reading", "sports"},
	}
	after := map[string]any{
		"name": "Alice",
		"details": map[string]any{
			"age":  31,
			"city": "NYC",
		},
		"hobbies": []any{"reading", "travel"},
	}
	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, patch, 1, "Expected one patch operation for array element replacement")

	operation := patch[0]
	assert.Equal(t, "replace", operation.Op)
	assert.Equal(t, "travel", operation.Value)
}

func TestGeneratePatch_ArrayRemove(t *testing.T) {
	// Arrange: an element is removed from the middle of the array.
	before := map[string]any{
		"list": []any{1, 2, 3, 4},
	}
	after := map[string]any{
		"list": []any{1, 2, 4},
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	found := false
	for _, op := range patch {
		if op.Op == "remove" && strings.HasPrefix(op.Path, "/list/") {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected remove op for a missing array element")
}

func TestGeneratePatch_ArrayMove(t *testing.T) {
	// Arrange: array reordering (swapping first two elements).
	before := map[string]any{
		"list": []any{1, 2, 3},
	}
	after := map[string]any{
		"list": []any{2, 1, 3},
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	found := false
	for _, op := range patch {
		if op.Op == "move" && strings.HasPrefix(op.Path, "/list/") {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected move op for array reordering")
}

func TestApplyPatch_Basic(t *testing.T) {
	// Arrange
	before := map[string]any{
		"name": "Alice",
		"age":  30,
	}
	after := map[string]any{
		"name": "Alice",
		"age":  31,
		"city": "NYC",
	}
	patch, err := GeneratePatch(before, after, "")
	assert.NoError(t, err)

	// Act
	result, err := ApplyPatch(before, patch)

	// Assert: Use a type assertion to convert result to map.

	assert.NoError(t, err)

	resultBytes, _ := json.Marshal(result)
	afterBytes, _ := json.Marshal(after)
	assert.Equal(t, string(afterBytes), string(resultBytes), "After applying patch, the result should match the 'after' state")
}

func TestApplyPatch_Nested(t *testing.T) {
	// Arrange
	before := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"email": "alice@old.com",
		},
	}
	after := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"email": "alice@new.com",
			"age":   25,
		},
	}
	patch, err := GeneratePatch(before, after, "")
	assert.NoError(t, err)

	// Act
	result, err := ApplyPatch(before, patch)

	// Assert
	assert.NoError(t, err)

	resultBytes, _ := json.Marshal(result)
	afterBytes, _ := json.Marshal(after)
	assert.Equal(t, string(afterBytes), string(resultBytes), "Nested objects should be updated correctly")
}

func TestApplyPatch_Array(t *testing.T) {
	// Arrange
	before := map[string]any{
		"list": []any{1, 2, 3},
	}
	after := map[string]any{
		"list": []any{1, 2, 3, 4},
	}
	patch, err := GeneratePatch(before, after, "")
	assert.NoError(t, err)

	// Act
	result, err := ApplyPatch(before, patch)

	// Assert
	assert.NoError(t, err)
	resultBytes, _ := json.Marshal(result)
	afterBytes, _ := json.Marshal(after)
	assert.Equal(t, string(afterBytes), string(resultBytes), "Array should be updated correctly after applying the patch")
}

func TestApplyPatch_MoveOperation(t *testing.T) {
	// Arrange: move the value from key "first" to "third".
	before := map[string]any{
		"first":  "value1",
		"second": "value2",
	}
	patch := []Patch{
		{Op: "move", Path: "/third", From: "/first", Value: nil},
	}

	// Act
	result, err := ApplyPatch(before, patch)

	// Assert
	assert.NoError(t, err)

	_, exists := result["first"]
	assert.False(t, exists, "Field 'first' should be removed after move")
	val, exists := result["third"]
	assert.True(t, exists, "Field 'third' should exist after move")
	assert.Equal(t, "value1", val, "Field 'third' should have the moved value")
}

func TestGenerateThenApply_PreserveData(t *testing.T) {
	// Arrange: a complex document with nested objects and arrays.
	before := map[string]any{
		"name": "Alice",
		"details": map[string]any{
			"age":  30,
			"city": "NYC",
		},
		"hobbies": []any{"reading", "sports"},
	}
	after := map[string]any{
		"name": "Alice",
		"details": map[string]any{
			"age":     31, // updated
			"city":    "NYC",
			"country": "USA", // added
		},
		"hobbies": []any{"reading", "travel"}, // modified array
	}
	patch, err := GeneratePatch(before, after, "")
	require.NoError(t, err)

	// Act
	result, err := ApplyPatch(before, patch)

	// Assert
	assert.NoError(t, err)
	resultBytes, _ := json.Marshal(result)
	afterBytes, _ := json.Marshal(after)
	assert.Equal(t, string(afterBytes), string(resultBytes), "The final document should match the expected state")
}

func TestPatchMarshalRoundtrip(t *testing.T) {
	patch := []Patch{
		{Op: "move", Path: "/third", From: "/first", Value: nil},
	}

	// Act
	jsonBytes, err := json.Marshal(patch)
	require.NoError(t, err)

	var unmarshaledPatch []Patch
	err = json.Unmarshal(jsonBytes, &unmarshaledPatch)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, patch, unmarshaledPatch, "Unmarshaled patch should match the original")
}

func TestApplyPatchAndHydrate(t *testing.T) {
	// Arrange
	type Person struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	before := &Person{Name: "Alice", Email: "a@old.com"}
	after := &Person{Name: "Alice", Email: "a@new.com"}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)

	var result Person
	err = ApplyPatchAndHydrate(before, &result, patch)
	require.NoError(t, err)
	assert.Equal(t, after.Email, result.Email)
}

func TestGeneratePatch_NestedRemove(t *testing.T) {
	// Arrange
	before := map[string]any{
		"user": map[string]any{
			"name":  "Alice",
			"email": "alice@old.com",
		},
	}
	after := map[string]any{
		"user": map[string]any{
			"name": "Alice",
		},
	}
	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert

	assert.NoError(t, err)

	var found bool
	for _, op := range patch {
		if op.Op == "remove" && op.Path == "/user/email" {
			found = true
		}
	}
	assert.True(t, found, "Expected remove operation for /user/email")
}

func TestGeneratePatch_IgnoresUnexportedFields(t *testing.T) {
	// Arrange
	type Example struct {
		Public  string `json:"public"`
		private string // unexported
	}

	before := &Example{Public: "a", private: "x"}
	after := &Example{Public: "b", private: "x"}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, patch, 1)
	assert.Equal(t, "/public", patch[0].Path)
}
