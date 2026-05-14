package jsonpatch

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldGenerateNoPatchWhenNoChanges(t *testing.T) {
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

func TestShouldGeneratePatchForFlatStructureChanges(t *testing.T) {
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

func TestShouldGeneratePatchForNestedStructureChanges(t *testing.T) {
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

func TestShouldGenerateAddOperationWhenArrayElementAdded(t *testing.T) {
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

func TestGeneratePatchShouldReplaceArrayElementsGivenArrayChanges(t *testing.T) {
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

func TestGeneratePatchShouldRemoveArrayElementsGivenArrayChanges(t *testing.T) {
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

func TestGeneratePatchShouldMoveArrayElementsGivenSwappedElements(t *testing.T) {
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

func TestGeneratePatchShouldHandleCommonPrefixAndSuffixInArrays(t *testing.T) {
	// Arrange
	before := map[string]any{
		"list": []any{"a", "b", "c", "d", "e"},
	}
	after := map[string]any{
		"list": []any{"a", "b", "x", "y", "e"},
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, patch, 2)
	assert.Equal(t, "replace", patch[0].Op)
	assert.Equal(t, "/list/2", patch[0].Path)
	assert.Equal(t, "x", patch[0].Value)
	assert.Equal(t, "replace", patch[1].Op)
	assert.Equal(t, "/list/3", patch[1].Path)
	assert.Equal(t, "y", patch[1].Value)
}

func TestShouldApplyBasicPatchOperationsCorrectly(t *testing.T) {
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

func TestApplyPatchShouldHandleNestedStructuresCorrectly(t *testing.T) {
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

func TestApplyPatchShouldHandleArrayOperationsCorrectly(t *testing.T) {
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

func TestApplyPatchShouldHandleMoveOperationsCorrectly(t *testing.T) {
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

func TestShouldPreserveDataWhenGeneratingAndApplyingPatches(t *testing.T) {
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

func TestShouldMarshalAndUnmarshalPatchCorrectly(t *testing.T) {
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

func TestShouldApplyPatchAndHydrateStructCorrectly(t *testing.T) {
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

func TestShouldGenerateFieldLevelReplaceGivenUUIDFieldChange(t *testing.T) {
	// Arrange
	type Document struct {
		ID uuid.UUID `json:"id"`
	}

	before := Document{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")}
	after := Document{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222")}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []Patch{{
		Op:    "replace",
		Path:  "/id",
		Value: after.ID.String(),
	}}, patch)
}

func TestShouldGenerateFieldLevelReplaceGivenTimeFieldChange(t *testing.T) {
	// Arrange
	type Document struct {
		UpdatedAt time.Time `json:"updatedAt"`
	}

	before := Document{UpdatedAt: time.Date(2024, time.January, 15, 10, 30, 0, 0, time.UTC)}
	after := Document{UpdatedAt: time.Date(2024, time.January, 16, 12, 45, 0, 0, time.UTC)}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []Patch{{
		Op:    "replace",
		Path:  "/updatedAt",
		Value: after.UpdatedAt.Format(time.RFC3339Nano),
	}}, patch)
}

func TestShouldApplyPatchAndHydrateSpecialJSONTypesWithoutByteLevelPaths(t *testing.T) {
	// Arrange
	type Document struct {
		ID        uuid.UUID       `json:"id"`
		UpdatedAt time.Time       `json:"updatedAt"`
		Addr      netip.Addr      `json:"addr"`
		Payload   json.RawMessage `json:"payload"`
	}

	before := Document{
		ID:        uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UpdatedAt: time.Date(2024, time.January, 15, 10, 30, 0, 0, time.UTC),
		Addr:      netip.MustParseAddr("192.0.2.10"),
		Payload:   json.RawMessage(`{"enabled":false,"roles":["reader"]}`),
	}
	after := Document{
		ID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		UpdatedAt: time.Date(2024, time.January, 16, 12, 45, 0, 0, time.UTC),
		Addr:      netip.MustParseAddr("192.0.2.20"),
		Payload:   json.RawMessage(`{"enabled":true,"roles":["reader","writer"]}`),
	}

	// Act
	patch, err := GeneratePatch(before, after, "")
	require.NoError(t, err)

	var result Document
	err = ApplyPatchAndHydrate(before, &result, patch)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, after.ID, result.ID)
	assert.True(t, result.UpdatedAt.Equal(after.UpdatedAt))
	assert.Equal(t, after.Addr, result.Addr)
	assert.JSONEq(t, string(after.Payload), string(result.Payload))

	for _, operation := range patch {
		assert.False(t, strings.HasPrefix(operation.Path, "/id/"), "uuid.UUID should not be diffed byte-by-byte")
		assert.False(t, strings.HasPrefix(operation.Path, "/updatedAt/"), "time.Time should not be diffed as a nested struct")
		assert.False(t, strings.HasPrefix(operation.Path, "/addr/"), "netip.Addr should not collapse into an internal representation")
		assert.False(t, strings.HasPrefix(operation.Path, "/payload/0"), "json.RawMessage should not be diffed as raw bytes")
	}
	assert.NotEmpty(t, patch)
}

func TestGeneratePatchShouldRemoveNestedPropertiesCorrectly(t *testing.T) {
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

func TestGeneratePatchShouldIgnoreUnexportedFields(t *testing.T) {
	// Arrange
	type MyStruct struct {
		ExportedField   string
		unexportedField string
	}
	before := MyStruct{ExportedField: "hello", unexportedField: "world"}
	after := MyStruct{ExportedField: "hello2", unexportedField: "world2"}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, patch, 1, "Only exported field changes should be included")
	assert.Equal(t, "/ExportedField", patch[0].Path)
	assert.Equal(t, "replace", patch[0].Op)
	assert.Equal(t, "hello2", patch[0].Value)
}

func TestGeneratePatchShouldTreatEquivalentNumericArrayValuesAsEqual(t *testing.T) {
	// Arrange
	before := map[string]any{"list": []any{1}}
	after := map[string]any{"list": []any{1.0}}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, patch)
}

// Error handling and edge case tests
func TestShouldReturnErrorWhenGeneratingPatchWithInvalidBeforeData(t *testing.T) {
	// Arrange - use a type that can't be converted to map
	before := make(chan int)
	after := map[string]any{"key": "value"}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.Error(t, err, "Should error when before data cannot be converted to map")
	assert.Nil(t, patch, "Patch should be nil on error")
}

func TestShouldReturnErrorWhenGeneratingPatchWithInvalidAfterData(t *testing.T) {
	// Arrange - use a type that can't be converted to map
	before := map[string]any{"key": "value"}
	after := make(chan int)

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.Error(t, err, "Should error when after data cannot be converted to map")
	assert.Nil(t, patch, "Patch should be nil on error")
}

func TestShouldReturnErrorWhenApplyingPatchWithInvalidOriginal(t *testing.T) {
	// Arrange
	original := make(chan int) // Cannot be converted to map
	patches := []Patch{{Op: "add", Path: "/test", Value: "value"}}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when original cannot be converted to map")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldReturnErrorWhenApplyingUnsupportedOperation(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "invalid_op", Path: "/test", Value: "value"}}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error on unsupported operation")
	assert.Nil(t, result, "Result should be nil on error")
	assert.ErrorContains(t, err, "unsupported op: invalid_op")
}

func TestShouldReplaceDocumentRootWhenApplyingAddPatchWithEmptyPath(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "add", Path: "", Value: map[string]any{"fresh": "value"}}}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"fresh": "value"}, result)
}

func TestShouldReplaceDocumentRootWhenApplyingReplacePatchWithEmptyPath(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "replace", Path: "", Value: map[string]any{"fresh": "value"}}}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"fresh": "value"}, result)
}

func TestShouldTestDocumentRootWhenApplyingPatchWithEmptyPath(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "test", Path: "", Value: map[string]any{"key": "value"}}}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestShouldReturnErrorWhenReplacingDocumentRootWithScalar(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "replace", Path: "", Value: "value"}}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "document root must be an object")
}

func TestShouldReturnErrorWhenRemovingDocumentRoot(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "remove", Path: ""}}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "cannot remove document root")
}

func TestShouldReturnErrorWhenApplyingMoveWithEmptyFromPath(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "move", Path: "/newkey", From: ""}} // Empty from path

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error on empty from path")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldReturnErrorWhenHydratingIntoInvalidType(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "add", Path: "/newkey", Value: "newvalue"}}
	var invalidTarget chan int // Cannot unmarshal into channel

	// Act
	err := ApplyPatchAndHydrate(original, &invalidTarget, patches)

	// Assert
	assert.Error(t, err, "Should error when target type cannot be unmarshaled into")
}

func TestShouldHandlePointerToMapInToMap(t *testing.T) {
	// Arrange
	originalMap := map[string]any{"key": "value"}
	pointerToMap := &originalMap

	// Act
	result, err := toMap(pointerToMap)

	// Assert
	assert.NoError(t, err, "Should handle pointer to map")
	assert.Equal(t, originalMap, result, "Should return the dereferenced map")
}

func TestShouldReturnErrorWhenConvertingInvalidTypeToMap(t *testing.T) {
	// Arrange
	invalidData := []string{"not", "a", "map", "or", "struct"}

	// Act
	result, err := toMap(invalidData)

	// Assert
	assert.Error(t, err, "Should error when converting slice to map")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldReturnErrorWhenConvertingInvalidTypeToSlice(t *testing.T) {
	// Arrange
	invalidData := map[string]any{"not": "a slice"}

	// Act
	result, err := toSlice(invalidData)

	// Assert
	assert.Error(t, err, "Should error when converting map to slice")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldHandleComplexArrayOperations(t *testing.T) {
	// Arrange - test complex array manipulations
	before := map[string]any{
		"list": []any{
			map[string]any{"id": 1, "name": "first"},
			map[string]any{"id": 2, "name": "second"},
			map[string]any{"id": 3, "name": "third"},
		},
	}
	after := map[string]any{
		"list": []any{
			map[string]any{"id": 1, "name": "first_updated"}, // modified
			map[string]any{"id": 3, "name": "third"},         // moved
			map[string]any{"id": 4, "name": "fourth"},        // added
		},
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err, "Should handle complex array operations")
	assert.NotEmpty(t, patch, "Should generate patches for complex array changes")

	// Apply patch and verify result
	result, err := ApplyPatch(before, patch)
	assert.NoError(t, err)

	resultBytes, _ := json.Marshal(result)
	afterBytes, _ := json.Marshal(after)
	assert.JSONEq(t, string(afterBytes), string(resultBytes), "Complex array operations should be applied correctly")
}

func TestShouldHandleDeepNestedStructures(t *testing.T) {
	// Arrange - test deeply nested structures
	before := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"value": "original",
				},
			},
		},
	}
	after := map[string]any{
		"level1": map[string]any{
			"level2": map[string]any{
				"level3": map[string]any{
					"value":    "updated",
					"newfield": "added",
				},
			},
		},
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err, "Should handle deeply nested structures")
	assert.NotEmpty(t, patch, "Should generate patches for nested changes")

	// Apply patch and verify result
	result, err := ApplyPatch(before, patch)
	assert.NoError(t, err)

	resultBytes, _ := json.Marshal(result)
	afterBytes, _ := json.Marshal(after)
	assert.JSONEq(t, string(afterBytes), string(resultBytes), "Deeply nested changes should be applied correctly")
}

func TestShouldHandleArrayIndexOperations(t *testing.T) {
	// Arrange - test specific array index operations
	original := map[string]any{
		"items": []any{"a", "b", "c", "d"},
	}

	// Test add at specific index
	patches := []Patch{
		{Op: "add", Path: "/items/2", Value: "inserted"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err, "Should handle array index add operation")
	items, ok := result["items"].([]any)
	assert.True(t, ok, "Items should be an array")
	assert.Len(t, items, 5, "Array should have 5 elements after insertion")
	assert.Equal(t, "inserted", items[2], "Element should be inserted at correct index")
	assert.Equal(t, "c", items[3], "Existing elements should be shifted")
}

func TestShouldHandleRemoveFromArrayIndex(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"a", "b", "c", "d"},
	}
	patches := []Patch{
		{Op: "remove", Path: "/items/1"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err, "Should handle array index remove operation")
	items, ok := result["items"].([]any)
	assert.True(t, ok, "Items should be an array")
	assert.Len(t, items, 3, "Array should have 3 elements after removal")
	assert.Equal(t, "a", items[0], "First element should remain")
	assert.Equal(t, "c", items[1], "Elements should shift after removal")
	assert.Equal(t, "d", items[2], "Last element should shift")
}

func TestShouldHandleReplaceInArrayIndex(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"a", "b", "c", "d"},
	}
	patches := []Patch{
		{Op: "replace", Path: "/items/1", Value: "replaced"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err, "Should handle array index replace operation")
	items, ok := result["items"].([]any)
	assert.True(t, ok, "Items should be an array")
	assert.Len(t, items, 4, "Array should maintain same length")
	assert.Equal(t, "a", items[0], "Other elements should remain unchanged")
	assert.Equal(t, "replaced", items[1], "Element should be replaced")
	assert.Equal(t, "c", items[2], "Other elements should remain unchanged")
}

func TestShouldReturnErrorForInvalidArrayIndex(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"a", "b", "c"},
	}
	patches := []Patch{
		{Op: "add", Path: "/items/invalid_index", Value: "value"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error on invalid array index")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldReturnErrorForOutOfRangeArrayIndex(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"a", "b", "c"},
	}
	patches := []Patch{
		{Op: "remove", Path: "/items/10"}, // Index out of range
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error on out of range array index")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldHandleStructWithJSONTags(t *testing.T) {
	// Arrange
	type TestStruct struct {
		PublicField  string `json:"public"`
		IgnoredField string `json:"-"`
		RenamedField string `json:"renamed"`
	}

	before := TestStruct{
		PublicField:  "value1",
		IgnoredField: "ignored",
		RenamedField: "value2",
	}
	after := TestStruct{
		PublicField:  "updated1",
		IgnoredField: "still_ignored",
		RenamedField: "updated2",
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, patch, 2, "Should generate patches for public and renamed fields only")

	// Check that ignored field is not in the patch
	for _, p := range patch {
		assert.NotEqual(t, "/IgnoredField", p.Path, "Ignored field should not be in patch")
		assert.NotEqual(t, "/ignored", p.Path, "Ignored field should not be in patch")
	}
}

func TestShouldHandleDifferentDataTypes(t *testing.T) {
	// Arrange
	before := map[string]any{
		"string": "hello",
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"null":   nil,
	}
	after := map[string]any{
		"string": "world",
		"int":    84,
		"float":  6.28,
		"bool":   false,
		"null":   "not null anymore",
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, patch, 5, "Should generate patches for all changed fields")

	// Apply patch and verify
	result, err := ApplyPatch(before, patch)
	assert.NoError(t, err)

	resultBytes, _ := json.Marshal(result)
	afterBytes, _ := json.Marshal(after)
	assert.JSONEq(t, string(afterBytes), string(resultBytes))
}

func TestShouldHandleJSONTagsWithOmitEmpty(t *testing.T) {
	// Arrange
	type TestStruct struct {
		Name     string `json:"name"`
		Optional string `json:"optional,omitempty"`
		Ignored  string `json:"-"`
		Default  string // No JSON tag - should use field name
	}

	before := TestStruct{
		Name:     "test",
		Optional: "",
		Ignored:  "should_not_appear",
		Default:  "default_value",
	}
	after := TestStruct{
		Name:     "updated",
		Optional: "now_has_value",
		Ignored:  "still_should_not_appear",
		Default:  "updated_default",
	}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)

	// Should only have patches for name, optional, and Default fields
	// Ignored field should not appear
	fieldNames := make(map[string]bool)
	for _, p := range patch {
		fieldNames[p.Path] = true
	}

	assert.True(t, fieldNames["/name"], "Should have patch for name field")
	assert.True(t, fieldNames["/optional"], "Should have patch for optional field")
	assert.True(t, fieldNames["/Default"], "Should have patch for Default field (no json tag)")
	assert.False(t, fieldNames["/Ignored"], "Should not have patch for ignored field")
	assert.False(t, fieldNames["/ignored"], "Should not have patch for ignored field")
}

func TestShouldReturnErrorWhenInvalidJSONPathInPatch(t *testing.T) {
	// Arrange
	original := map[string]any{"name": "test"}
	patches := []Patch{
		{Op: "replace", Path: "/invalid/nested/path", Value: "new_value"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when trying to access invalid nested path")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldHandleEmptyStructs(t *testing.T) {
	// Arrange
	type EmptyStruct struct{}
	before := EmptyStruct{}
	after := EmptyStruct{}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, patch, 0, "Should generate no patches for identical empty structs")
}

// Tests for 0% coverage functions: insertIntoSlice, removeFromSlice, replaceInSlice, getFromSlice

func TestShouldInsertIntoSliceAtBeginning(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"b", "c", "d"},
	}
	patches := []Patch{
		{Op: "add", Path: "/items/0", Value: "a"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	items := result["items"].([]any)
	assert.Equal(t, []any{"a", "b", "c", "d"}, items, "Should insert at beginning of array")
}

func TestShouldInsertIntoSliceAtMiddle(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"a", "c", "d"},
	}
	patches := []Patch{
		{Op: "add", Path: "/items/1", Value: "b"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	items := result["items"].([]any)
	assert.Equal(t, []any{"a", "b", "c", "d"}, items, "Should insert at middle of array")
}

func TestShouldInsertIntoNonExistentSlice(t *testing.T) {
	// Arrange
	original := map[string]any{}
	patches := []Patch{
		{Op: "add", Path: "/items", Value: []any{"first"}}, // Add the array first
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	items := result["items"].([]any)
	assert.Equal(t, []any{"first"}, items, "Should create new slice when adding array to non-existent field")
}

func TestShouldReturnErrorWhenInsertingAtInvalidIndex(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"a", "b"},
	}
	patches := []Patch{
		{Op: "add", Path: "/items/-1", Value: "invalid"}, // Negative index
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error on negative index")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "index")
}

func TestShouldReturnErrorWhenInsertingBeyondSliceLength(t *testing.T) {
	// Arrange
	original := map[string]any{
		"items": []any{"a", "b"},
	}
	patches := []Patch{
		{Op: "add", Path: "/items/5", Value: "invalid"}, // Index too large
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when index exceeds slice length")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "index")
}

func TestShouldRemoveFromSliceUsingComplexPath(t *testing.T) {
	// Arrange
	original := map[string]any{
		"data": map[string]any{
			"nested": map[string]any{
				"items": []any{"a", "b", "c", "d"},
			},
		},
	}
	patches := []Patch{
		{Op: "remove", Path: "/data/nested/items/1"}, // Remove "b"
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	nestedItems := result["data"].(map[string]any)["nested"].(map[string]any)["items"].([]any)
	assert.Equal(t, []any{"a", "c", "d"}, nestedItems, "Should remove element from nested slice")
}

func TestShouldReturnErrorWhenRemovingFromInvalidSliceIndex(t *testing.T) {
	// Arrange
	original := map[string]any{
		"data": map[string]any{
			"items": []any{"a", "b"},
		},
	}
	patches := []Patch{
		{Op: "remove", Path: "/data/items/5"}, // Index out of range
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when removing from invalid index")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "invalid index 5")
}

func TestShouldReturnErrorWhenRemovingFromNonSlice(t *testing.T) {
	// Arrange
	original := map[string]any{
		"data": map[string]any{
			"items": "not_a_slice",
		},
	}
	patches := []Patch{
		{Op: "remove", Path: "/data/items/0"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when trying to remove from non-slice")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "unexpected type at items")
}

func TestShouldReplaceInSliceUsingComplexPath(t *testing.T) {
	// Arrange
	original := map[string]any{
		"data": map[string]any{
			"nested": map[string]any{
				"items": []any{"a", "b", "c", "d"},
			},
		},
	}
	patches := []Patch{
		{Op: "replace", Path: "/data/nested/items/1", Value: "replaced"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	nestedItems := result["data"].(map[string]any)["nested"].(map[string]any)["items"].([]any)
	assert.Equal(t, []any{"a", "replaced", "c", "d"}, nestedItems, "Should replace element in nested slice")
}

func TestShouldReturnErrorWhenReplacingInInvalidSliceIndex(t *testing.T) {
	// Arrange
	original := map[string]any{
		"data": map[string]any{
			"items": []any{"a", "b"},
		},
	}
	patches := []Patch{
		{Op: "replace", Path: "/data/items/5", Value: "replacement"}, // Index out of range
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when replacing at invalid index")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "invalid index 5")
}

func TestShouldReturnErrorWhenReplacingInNonSlice(t *testing.T) {
	// Arrange
	original := map[string]any{
		"data": map[string]any{
			"items": "not_a_slice",
		},
	}
	patches := []Patch{
		{Op: "replace", Path: "/data/items/0", Value: "replacement"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when trying to replace in non-slice")
	assert.Nil(t, result)
	assert.ErrorContains(t, err, "unexpected type at items")
}

func TestShouldMoveElementFromComplexSlicePath(t *testing.T) {
	// Arrange
	original := map[string]any{
		"source": map[string]any{
			"items": []any{"move_me", "stay"},
		},
		"target": map[string]any{
			"items": []any{"existing"},
		},
	}
	patches := []Patch{
		{Op: "move", Path: "/target/items/1", From: "/source/items/0"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	sourceItems := result["source"].(map[string]any)["items"].([]any)
	targetItems := result["target"].(map[string]any)["items"].([]any)
	assert.Equal(t, []any{"stay"}, sourceItems, "Should remove element from source slice")
	assert.Equal(t, []any{"existing", "move_me"}, targetItems, "Should add element to target slice")
}

func TestShouldHandleGetFromSliceWithInvalidIndex(t *testing.T) {
	// Arrange - this will test the getFromSlice function via move operation
	original := map[string]any{
		"data": map[string]any{
			"items": []any{"a", "b"},
		},
		"target": "placeholder",
	}
	patches := []Patch{
		{Op: "move", Path: "/target", From: "/data/items/5"}, // Invalid index
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when getting from invalid slice index")
	assert.Nil(t, result)
}

func TestShouldHandleGetFromSliceWithNonSlice(t *testing.T) {
	// Arrange - this will test the getFromSlice function via move operation
	original := map[string]any{
		"data": map[string]any{
			"items": "not_a_slice",
		},
		"target": "placeholder",
	}
	patches := []Patch{
		{Op: "move", Path: "/target", From: "/data/items/0"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when getting from non-slice")
	assert.Nil(t, result)
}

// Tests for low coverage functions: toSlice, applyMove, parseIndexFromPath, ApplyPatchAndHydrate

func TestShouldConvertTypedSliceToAnySlice(t *testing.T) {
	// Arrange
	typedSlice := []string{"a", "b", "c"}
	before := map[string]any{"items": typedSlice}
	after := map[string]any{"items": []string{"a", "b", "c", "d"}}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err, "Should handle typed slice conversion via toSlice function")
	assert.NotEmpty(t, patch, "Should generate patch for typed slice")
}

func TestShouldConvertTypedArrayToAnySlice(t *testing.T) {
	// Arrange
	typedArray := [3]string{"a", "b", "c"}
	before := map[string]any{"items": typedArray}
	after := map[string]any{"items": [4]string{"a", "b", "c", "d"}}

	// Act
	patch, err := GeneratePatch(before, after, "")

	// Assert
	assert.NoError(t, err, "Should handle typed array conversion via toSlice function")
	assert.NotEmpty(t, patch, "Should generate patch for typed array")
}

func TestShouldReturnErrorWhenParsingIndexFromEmptyPath(t *testing.T) {
	// Arrange
	original := map[string]any{"items": []any{"a"}}
	patches := []Patch{
		{Op: "add", Path: "/", Value: "test"}, // Empty path parts
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error on empty path when parsing index")
	assert.Nil(t, result)
}

func TestShouldReturnErrorWhenParsingNonNumericIndex(t *testing.T) {
	// Arrange
	original := map[string]any{"items": []any{"a"}}
	patches := []Patch{
		{Op: "add", Path: "/items/not_a_number", Value: "test"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when path contains non-numeric index")
	assert.Nil(t, result)
}

func TestShouldReturnErrorWhenMovingFromNonExistentPath(t *testing.T) {
	// Arrange
	original := map[string]any{"existing": "value"}
	patches := []Patch{
		{Op: "move", Path: "/target", From: "/non_existent"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when moving from non-existent path")
	assert.Nil(t, result)
}

func TestShouldReturnErrorWhenMovingToInvalidPath(t *testing.T) {
	// Arrange
	original := map[string]any{"source": "value"}
	patches := []Patch{
		{Op: "move", Path: "/invalid/nested/path", From: "/source"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error when moving to invalid nested path")
	assert.Nil(t, result)
}

func TestShouldReturnErrorInApplyPatchAndHydrateWhenMarshalFails(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "add", Path: "/channel", Value: make(chan int)}} // Unmarshalable type
	var target map[string]any

	// Act
	err := ApplyPatchAndHydrate(original, &target, patches)

	// Assert
	assert.Error(t, err, "Should error when marshal fails due to unmarshalable type")
	assert.ErrorContains(t, err, "marshal")
}

func TestShouldReturnErrorInApplyPatchAndHydrateWhenApplyPatchFails(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "invalid_op", Path: "/test", Value: "value"}}
	var target map[string]any

	// Act
	err := ApplyPatchAndHydrate(original, &target, patches)

	// Assert
	assert.Error(t, err, "Should error when ApplyPatch fails")
	assert.ErrorContains(t, err, "apply patch")
}

func TestShouldHandleMoveOperationWithComplexPaths(t *testing.T) {
	// Arrange - test move operation between different complex paths
	original := map[string]any{
		"deep": map[string]any{
			"nested": map[string]any{
				"source": []any{"move_this", "keep_this"},
			},
		},
		"other": map[string]any{
			"target": []any{"existing"},
		},
	}
	patches := []Patch{
		{Op: "move", Path: "/other/target/0", From: "/deep/nested/source/0"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	sourceSlice := result["deep"].(map[string]any)["nested"].(map[string]any)["source"].([]any)
	targetSlice := result["other"].(map[string]any)["target"].([]any)

	assert.Equal(t, []any{"keep_this"}, sourceSlice, "Source should have element removed")
	assert.Equal(t, []any{"move_this", "existing"}, targetSlice, "Target should have element inserted at beginning")
}

func TestShouldHandleSuccessfulHydrateWithComplexStructure(t *testing.T) {
	// Arrange
	type NestedStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	type ComplexStruct struct {
		User   NestedStruct `json:"user"`
		Active bool         `json:"active"`
	}

	original := ComplexStruct{
		User:   NestedStruct{Name: "Alice", Age: 30},
		Active: false,
	}
	patches := []Patch{
		{Op: "replace", Path: "/user/age", Value: 31},
		{Op: "replace", Path: "/active", Value: true},
	}

	var result ComplexStruct

	// Act
	err := ApplyPatchAndHydrate(original, &result, patches)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 31, result.User.Age, "Should update nested field")
	assert.True(t, result.Active, "Should update boolean field")
	assert.Equal(t, "Alice", result.User.Name, "Should preserve unchanged fields")
}

// ============================================================================
// RFC 6902 JSON Patch Compliance Tests
// ============================================================================

func TestShouldAddPropertyToObjectGivenRFC6902AddOperation(t *testing.T) {
	// Arrange - RFC 6902 Section 4.1: Add Operation
	original := map[string]any{
		"foo": "bar",
	}
	patches := []Patch{
		{Op: "add", Path: "/baz", Value: "qux"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "bar", result["foo"], "Original property should be preserved")
	assert.Equal(t, "qux", result["baz"], "New property should be added")
}

func TestShouldAddElementToArray(t *testing.T) {
	// Arrange - RFC 6902 Section 4.1: Add to array
	original := map[string]any{
		"foo": []any{"bar", "baz"},
	}
	patches := []Patch{
		{Op: "add", Path: "/foo/1", Value: "qux"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	arr := result["foo"].([]any)
	assert.Equal(t, []any{"bar", "qux", "baz"}, arr, "Element should be inserted at specified index")
}

func TestShouldAppendToArrayWhenIndexEqualsLength(t *testing.T) {
	// Arrange - Adding to end of array using index equal to array length
	original := map[string]any{
		"foo": []any{"bar", "baz"},
	}
	patches := []Patch{
		{Op: "add", Path: "/foo/2", Value: "qux"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	arr := result["foo"].([]any)
	assert.Equal(t, []any{"bar", "baz", "qux"}, arr, "Element should be appended when index equals array length")
}

func TestShouldCreateNestedStructure(t *testing.T) {
	// Arrange - Adding to nested paths
	original := map[string]any{
		"foo": map[string]any{
			"bar": "baz",
		},
	}
	patches := []Patch{
		{Op: "add", Path: "/foo/qux", Value: "quux"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	nested := result["foo"].(map[string]any)
	assert.Equal(t, "baz", nested["bar"], "Existing nested property should be preserved")
	assert.Equal(t, "quux", nested["qux"], "New nested property should be added")
}

func TestShouldRemovePropertyFromObject(t *testing.T) {
	// Arrange - Remove Operation
	original := map[string]any{
		"foo": "bar",
		"baz": "qux",
	}
	patches := []Patch{
		{Op: "remove", Path: "/baz"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "bar", result["foo"], "Remaining property should be preserved")
	_, exists := result["baz"]
	assert.False(t, exists, "Removed property should not exist")
}

func TestShouldRemoveElementFromArray(t *testing.T) {
	// Arrange - Remove from array
	original := map[string]any{
		"foo": []any{"bar", "qux", "baz"},
	}
	patches := []Patch{
		{Op: "remove", Path: "/foo/1"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	arr := result["foo"].([]any)
	assert.Equal(t, []any{"bar", "baz"}, arr, "Element should be removed and array compacted")
}

func TestShouldReplacePropertyValue(t *testing.T) {
	// Arrange - Replace Operation
	original := map[string]any{
		"foo": "bar",
		"baz": "qux",
	}
	patches := []Patch{
		{Op: "replace", Path: "/baz", Value: "boo"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "bar", result["foo"], "Other properties should be preserved")
	assert.Equal(t, "boo", result["baz"], "Property value should be replaced")
}

func TestShouldReplaceArrayElement(t *testing.T) {
	// Arrange - Replace array element
	original := map[string]any{
		"foo": []any{"bar", "baz"},
	}
	patches := []Patch{
		{Op: "replace", Path: "/foo/1", Value: "qux"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	arr := result["foo"].([]any)
	assert.Equal(t, []any{"bar", "qux"}, arr, "Array element should be replaced")
}

func TestShouldMoveValueBetweenProperties(t *testing.T) {
	// Arrange - Move Operation
	original := map[string]any{
		"foo": map[string]any{
			"bar":   "baz",
			"waldo": "fred",
		},
		"qux": map[string]any{
			"corge": "grault",
		},
	}
	patches := []Patch{
		{Op: "move", Path: "/qux/thud", From: "/foo/waldo"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	foo := result["foo"].(map[string]any)
	qux := result["qux"].(map[string]any)

	assert.Equal(t, "baz", foo["bar"], "Remaining source properties should be preserved")
	_, exists := foo["waldo"]
	assert.False(t, exists, "Moved property should be removed from source")

	assert.Equal(t, "grault", qux["corge"], "Existing target properties should be preserved")
	assert.Equal(t, "fred", qux["thud"], "Property should be moved to target location")
}

func TestShouldMoveArrayElement(t *testing.T) {
	// Arrange - Move array element
	original := map[string]any{
		"foo": []any{"all", "grass", "cows", "eat"},
	}
	patches := []Patch{
		{Op: "move", Path: "/foo/3", From: "/foo/1"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	arr := result["foo"].([]any)
	assert.Equal(t, []any{"all", "cows", "eat", "grass"}, arr, "Array element should be moved to new position")
}

func TestShouldFailOnNonExistentPath(t *testing.T) {
	// Arrange - Operations on non-existent paths should fail
	original := map[string]any{
		"foo": "bar",
	}
	patches := []Patch{
		{Op: "remove", Path: "/nonexistent"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert - RFC 6902 compliant behavior
	assert.Error(t, err, "Operation on non-existent path should fail")
	assert.Nil(t, result, "Result should be nil on error")
	assert.ErrorContains(t, err, "does not exist", "Error should indicate path doesn't exist")
}

func TestShouldFailOnReplaceNonExistentPath(t *testing.T) {
	// Arrange - Replace operations on non-existent paths should fail
	original := map[string]any{
		"foo": "bar",
	}
	patches := []Patch{
		{Op: "replace", Path: "/nonexistent", Value: "new_value"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert - RFC 6902 compliant behavior
	assert.Error(t, err, "Replace operation on non-existent path should fail")
	assert.Nil(t, result, "Result should be nil on error")
	assert.ErrorContains(t, err, "does not exist", "Error should indicate path doesn't exist")
}

func TestShouldFailOnInvalidArrayIndex(t *testing.T) {
	// Arrange - Invalid array indices should fail
	original := map[string]any{
		"foo": []any{"bar", "baz"},
	}
	patches := []Patch{
		{Op: "remove", Path: "/foo/invalid"},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Invalid array index should fail")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldFailOnOutOfBoundsArrayIndex(t *testing.T) {
	// Arrange - Out of bounds array access should fail (except for add at end)
	original := map[string]any{
		"foo": []any{"bar", "baz"},
	}
	patches := []Patch{
		{Op: "remove", Path: "/foo/5"}, // Index 5 is out of bounds
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Out of bounds array access should fail")
	assert.Nil(t, result, "Result should be nil on error")
}

func TestShouldFailEntirelyOrSucceedEntirely(t *testing.T) {
	// Arrange - Patch application should be atomic
	original := map[string]any{
		"foo": "bar",
		"baz": []any{"qux"},
	}
	patches := []Patch{
		{Op: "replace", Path: "/foo", Value: "new_bar"}, // Valid operation
		{Op: "remove", Path: "/nonexistent"},            // Invalid operation
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert - RFC 6902 compliant atomicity
	assert.Error(t, err, "Patch with invalid operation should fail entirely")
	assert.Nil(t, result, "No partial changes should be applied on failure")

	// Original should be unchanged (deep copy ensures this)
	assert.Equal(t, "bar", original["foo"], "Original document should remain unchanged")
}

func TestShouldHandleNestedOperations(t *testing.T) {
	// Arrange - Complex document with nested operations
	original := map[string]any{
		"foo": map[string]any{
			"bar": []any{
				map[string]any{"baz": "qux"},
				map[string]any{"hello": "world"},
			},
		},
		"array": []any{1, 2, 3},
	}
	patches := []Patch{
		{Op: "replace", Path: "/foo/bar/0/baz", Value: "new_qux"},
		{Op: "add", Path: "/foo/bar/1/new_prop", Value: "new_value"},
		{Op: "remove", Path: "/array/1"}, // Remove "2"
		{Op: "add", Path: "/foo/new_array", Value: []any{"a", "b"}},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)

	// Verify nested object changes
	fooBar := result["foo"].(map[string]any)["bar"].([]any)
	firstItem := fooBar[0].(map[string]any)
	secondItem := fooBar[1].(map[string]any)

	assert.Equal(t, "new_qux", firstItem["baz"], "Nested object property should be replaced")
	assert.Equal(t, "world", secondItem["hello"], "Existing nested property should be preserved")
	assert.Equal(t, "new_value", secondItem["new_prop"], "New nested property should be added")

	// Verify array changes
	resultArray := result["array"].([]any)
	assert.Equal(t, []any{1, 3}, resultArray, "Array element should be removed")

	// Verify new array addition
	newArray := result["foo"].(map[string]any)["new_array"].([]any)
	assert.Equal(t, []any{"a", "b"}, newArray, "New array should be added")
}

func TestShouldPreserveJSONTypes(t *testing.T) {
	// Arrange - Should preserve JSON data types
	original := map[string]any{
		"string":  "hello",
		"number":  42,
		"float":   3.14,
		"boolean": true,
		"null":    nil,
		"object":  map[string]any{"nested": "value"},
		"array":   []any{1, "two", false},
	}
	patches := []Patch{
		{Op: "replace", Path: "/string", Value: "world"},
		{Op: "replace", Path: "/number", Value: 84},
		{Op: "replace", Path: "/float", Value: 6.28},
		{Op: "replace", Path: "/boolean", Value: false},
		{Op: "replace", Path: "/null", Value: "not null"},
		{Op: "add", Path: "/object/new", Value: 123},
		{Op: "add", Path: "/array/3", Value: nil},
	}

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "world", result["string"])
	assert.Equal(t, 84, result["number"])
	assert.Equal(t, 6.28, result["float"])
	assert.Equal(t, false, result["boolean"])
	assert.Equal(t, "not null", result["null"])

	obj := result["object"].(map[string]any)
	assert.Equal(t, "value", obj["nested"])
	assert.Equal(t, 123, obj["new"])

	arr := result["array"].([]any)
	assert.Equal(t, []any{1, "two", false, nil}, arr)
}

// --- Table-driven ApplyPatch edge cases (plan §1) ---

type applyCase struct {
	name        string
	doc         map[string]any
	patch       []Patch
	wantErr     bool
	errContains string
	wantDoc     map[string]any
}

func TestShouldApplyPatchCorrectlyGivenEdgeCaseInputsWhenApplyingPatches(t *testing.T) {
	cases := []applyCase{
		// Empty/nil patch list
		{
			name:    "Empty_patch_list",
			doc:     map[string]any{"foo": "bar"},
			patch:   []Patch{},
			wantErr: false,
			wantDoc: map[string]any{"foo": "bar"},
		},
		{
			name:    "Nil_patch_list",
			doc:     map[string]any{"foo": "bar"},
			patch:   nil,
			wantErr: false,
			wantDoc: map[string]any{"foo": "bar"},
		},
		// Path format
		{
			name:        "Empty_path_requires_object_root_value",
			doc:         map[string]any{"foo": "bar"},
			patch:       []Patch{{Op: "add", Path: "", Value: "x"}},
			wantErr:     true,
			errContains: "document root must be an object",
		},
		{
			name:    "Add_at_root",
			doc:     map[string]any{"foo": "bar"},
			patch:   []Patch{{Op: "add", Path: "/baz", Value: "qux"}},
			wantErr: false,
			wantDoc: map[string]any{"foo": "bar", "baz": "qux"},
		},
		{
			name:    "Append_index_dash",
			doc:     map[string]any{"arr": []any{1, 2}},
			patch:   []Patch{{Op: "add", Path: "/arr/-", Value: 3}},
			wantErr: false,
			wantDoc: map[string]any{"arr": []any{1, 2, 3}},
		},
		// Add
		{
			name:        "Add_to_non_existent_parent",
			doc:         map[string]any{"foo": "bar"},
			patch:       []Patch{{Op: "add", Path: "/baz/bat", Value: "qux"}},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:        "Add_at_index_past_length",
			doc:         map[string]any{"arr": []any{1, 2}},
			patch:       []Patch{{Op: "add", Path: "/arr/10", Value: 3}},
			wantErr:     true,
			errContains: "index",
		},
		{
			name:    "Add_at_index_equals_length",
			doc:     map[string]any{"arr": []any{1, 2}},
			patch:   []Patch{{Op: "add", Path: "/arr/2", Value: 3}},
			wantErr: false,
			wantDoc: map[string]any{"arr": []any{1, 2, 3}},
		},
		// Remove
		{
			name:        "Remove_non_existent_path",
			doc:         map[string]any{"foo": "bar"},
			patch:       []Patch{{Op: "remove", Path: "/nonexistent"}},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:    "Remove_array_index_out_of_bounds",
			doc:     map[string]any{"arr": []any{1, 2}},
			patch:   []Patch{{Op: "remove", Path: "/arr/5"}},
			wantErr: true,
		},
		{
			name:    "Remove_root_level_key",
			doc:     map[string]any{"foo": "bar", "baz": "qux"},
			patch:   []Patch{{Op: "remove", Path: "/baz"}},
			wantErr: false,
			wantDoc: map[string]any{"foo": "bar"},
		},
		// Replace
		{
			name:        "Replace_non_existent_path",
			doc:         map[string]any{"foo": "bar"},
			patch:       []Patch{{Op: "replace", Path: "/nonexistent", Value: "x"}},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:    "Replace_at_valid_path_with_nil",
			doc:     map[string]any{"foo": "bar"},
			patch:   []Patch{{Op: "replace", Path: "/foo", Value: nil}},
			wantErr: false,
			wantDoc: map[string]any{"foo": nil},
		},
		// Move
		{
			name:        "Move_from_non_existent",
			doc:         map[string]any{"foo": "bar"},
			patch:       []Patch{{Op: "move", From: "/missing", Path: "/here"}},
			wantErr:     true,
			errContains: "does not exist",
		},
		{
			name:        "Move_from_prefix_of_path",
			doc:         map[string]any{"foo": map[string]any{"bar": "baz"}},
			patch:       []Patch{{Op: "move", From: "/foo", Path: "/foo/qux"}},
			wantErr:     true,
			errContains: "prefix",
		},
		// Atomicity: second op fails
		{
			name: "Second_op_fails_atomicity",
			doc:  map[string]any{"foo": "bar", "baz": []any{1}},
			patch: []Patch{
				{Op: "replace", Path: "/foo", Value: "new"},
				{Op: "remove", Path: "/nonexistent"},
			},
			wantErr:     true,
			errContains: "does not exist",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			doc := c.doc
			patches := c.patch

			// Act
			result, err := ApplyPatch(doc, patches)

			// Assert
			if c.wantErr {
				require.Error(t, err)
				if c.errContains != "" {
					assert.Contains(t, err.Error(), c.errContains)
				}
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			if c.wantDoc != nil {
				assert.Equal(t, c.wantDoc, result)
			}
		})
	}
}

// --- RFC 6902 spec-style inline cases (plan §2) ---

func TestShouldApplyPatchCorrectlyGivenRFC6902SpecCasesWhenApplyingPatches(t *testing.T) {
	cases := []applyCase{
		// A.1 Adding an Object Member
		{
			name:    "A.1_Adding_Object_Member",
			doc:     map[string]any{"foo": "bar"},
			patch:   []Patch{{Op: "add", Path: "/baz", Value: "qux"}},
			wantErr: false,
			wantDoc: map[string]any{"baz": "qux", "foo": "bar"},
		},
		// A.2 Adding an Array Element
		{
			name:    "A.2_Adding_Array_Element",
			doc:     map[string]any{"foo": []any{"bar", "baz"}},
			patch:   []Patch{{Op: "add", Path: "/foo/1", Value: "qux"}},
			wantErr: false,
			wantDoc: map[string]any{"foo": []any{"bar", "qux", "baz"}},
		},
		// A.3 Removing an Object Member
		{
			name:    "A.3_Removing_Object_Member",
			doc:     map[string]any{"baz": "qux", "foo": "bar"},
			patch:   []Patch{{Op: "remove", Path: "/baz"}},
			wantErr: false,
			wantDoc: map[string]any{"foo": "bar"},
		},
		// A.4 Removing an Array Element
		{
			name:    "A.4_Removing_Array_Element",
			doc:     map[string]any{"foo": []any{"bar", "qux", "baz"}},
			patch:   []Patch{{Op: "remove", Path: "/foo/1"}},
			wantErr: false,
			wantDoc: map[string]any{"foo": []any{"bar", "baz"}},
		},
		// A.5 Replacing a Value
		{
			name:    "A.5_Replacing_Value",
			doc:     map[string]any{"baz": "qux", "foo": "bar"},
			patch:   []Patch{{Op: "replace", Path: "/baz", Value: "boo"}},
			wantErr: false,
			wantDoc: map[string]any{"baz": "boo", "foo": "bar"},
		},
		// A.6 Moving a Value
		{
			name: "A.6_Moving_Value",
			doc: map[string]any{
				"foo": map[string]any{"bar": "baz", "waldo": "fred"},
				"qux": map[string]any{"corge": "grault"},
			},
			patch:   []Patch{{Op: "move", From: "/foo/waldo", Path: "/qux/thud"}},
			wantErr: false,
			wantDoc: map[string]any{
				"foo": map[string]any{"bar": "baz"},
				"qux": map[string]any{"corge": "grault", "thud": "fred"},
			},
		},
		// A.7 Moving an Array Element
		{
			name:    "A.7_Moving_Array_Element",
			doc:     map[string]any{"foo": []any{"all", "grass", "cows", "eat"}},
			patch:   []Patch{{Op: "move", From: "/foo/1", Path: "/foo/3"}},
			wantErr: false,
			wantDoc: map[string]any{"foo": []any{"all", "cows", "eat", "grass"}},
		},
		// A.10 Adding a nested member object
		{
			name:    "A.10_Adding_Nested_Object",
			doc:     map[string]any{"foo": "bar"},
			patch:   []Patch{{Op: "add", Path: "/child", Value: map[string]any{"grandchild": map[string]any{}}}},
			wantErr: false,
			wantDoc: map[string]any{
				"foo":   "bar",
				"child": map[string]any{"grandchild": map[string]any{}},
			},
		},
		// A.12 Adding to a non-existent target
		{
			name:        "A.12_Add_to_non_existent_target",
			doc:         map[string]any{"foo": "bar"},
			patch:       []Patch{{Op: "add", Path: "/baz/bat", Value: "qux"}},
			wantErr:     true,
			errContains: "does not exist",
		},
		// 4.1 add with missing object
		{
			name:        "4.1_Add_with_missing_object",
			doc:         map[string]any{"q": map[string]any{"bar": 2}},
			patch:       []Patch{{Op: "add", Path: "/a/b", Value: 1}},
			wantErr:     true,
			errContains: "does not exist",
		},
		// A.16 Adding with path /foo/- (append) per RFC 6902
		{
			name:    "A.16_Add_append_index_dash",
			doc:     map[string]any{"foo": []any{"bar"}},
			patch:   []Patch{{Op: "add", Path: "/foo/-", Value: []any{"abc", "def"}}},
			wantErr: false,
			wantDoc: map[string]any{"foo": []any{"bar", []any{"abc", "def"}}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			doc := c.doc
			patches := c.patch

			// Act
			result, err := ApplyPatch(doc, patches)

			// Assert
			if c.wantErr {
				require.Error(t, err)
				if c.errContains != "" {
					assert.Contains(t, err.Error(), c.errContains)
				}
				assert.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			if c.wantDoc != nil {
				assert.Equal(t, c.wantDoc, result)
			}
		})
	}
}

// --- GeneratePatch round-trip and edge cases (plan §3) ---

func TestShouldProduceAfterDocumentGivenBeforeAndAfterWhenGenerateAndApplyPatch(t *testing.T) {
	tests := []struct {
		name   string
		before map[string]any
		after  map[string]any
	}{
		{
			name:   "Empty_object_to_object",
			before: map[string]any{},
			after:  map[string]any{"a": 1},
		},
		{
			name:   "Empty_array",
			before: map[string]any{"arr": []any{}},
			after:  map[string]any{"arr": []any{1}},
		},
		{
			name:   "Nil_value_replaced",
			before: map[string]any{"x": nil},
			after:  map[string]any{"x": "not nil"},
		},
		{
			name:   "Nested_object_change",
			before: map[string]any{"a": map[string]any{"b": 1}},
			after:  map[string]any{"a": map[string]any{"b": 2, "c": 3}},
		},
		{
			name:   "Array_swap_adjacent",
			before: map[string]any{"list": []any{1, 2, 3}},
			after:  map[string]any{"list": []any{2, 1, 3}},
		},
		{
			name:   "Array_insert_and_delete",
			before: map[string]any{"list": []any{1, 2, 3}},
			after:  map[string]any{"list": []any{1, 99, 3}},
		},
		{
			name:   "Different_top_level_keys",
			before: map[string]any{"old": true},
			after:  map[string]any{"new": true},
		},
		{
			name:   "Deep_nesting",
			before: map[string]any{"a": map[string]any{"b": map[string]any{"c": 1}}},
			after:  map[string]any{"a": map[string]any{"b": map[string]any{"c": 2, "d": 4}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			before := tt.before
			after := tt.after

			// Act
			patches, err := GeneratePatch(before, after, "")
			require.NoError(t, err)
			result, err := ApplyPatch(before, patches)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, after, result)
		})
	}
}

func TestShouldGeneratePatchCorrectlyGivenEdgeCaseInputsWhenComparingBeforeAndAfter(t *testing.T) {
	t.Run("Empty_map_no_change", func(t *testing.T) {
		// Arrange
		before := map[string]any{}
		after := map[string]any{}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Empty(t, patches)
		assert.Equal(t, after, result)
	})
	t.Run("Slice_with_nil_elements", func(t *testing.T) {
		// Arrange
		before := map[string]any{"arr": []any{1, nil, 3}}
		after := map[string]any{"arr": []any{1, nil, 99}}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Non_empty_basePath", func(t *testing.T) {
		// Arrange
		beforeRoot := map[string]any{"a": 1}
		afterRoot := map[string]any{"a": 2}
		before := map[string]any{"root": beforeRoot}
		after := map[string]any{"root": afterRoot}

		// Act
		patches, err := GeneratePatch(beforeRoot, afterRoot, "/root")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, patches)
		for _, p := range patches {
			assert.True(t, strings.HasPrefix(p.Path, "/root/"), "path should have basePath prefix: %s", p.Path)
		}
		assert.Equal(t, after, result)
	})
	t.Run("Type_change_object_to_array", func(t *testing.T) {
		// Arrange
		before := map[string]any{"x": map[string]any{"a": 1}}
		after := map[string]any{"x": []any{1, 2}}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, patches)
		assert.Equal(t, after, result)
	})
}

func TestShouldReturnErrorGivenInvalidInputsWhenGeneratePatch(t *testing.T) {
	t.Run("Invalid_before_not_map_or_struct", func(t *testing.T) {
		// Arrange
		before := "not a map"
		after := map[string]any{"a": 1}

		// Act
		_, err := GeneratePatch(before, after, "")

		// Assert
		require.Error(t, err)
	})
	t.Run("Invalid_after_not_map_or_struct", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a": 1}
		after := "not a map"

		// Act
		_, err := GeneratePatch(before, after, "")

		// Assert
		require.Error(t, err)
	})
}

// --- Nil and null edge cases ---

func TestShouldHandleNilAndNullCorrectlyGivenVariousInputsWhenApplyingOrGeneratingPatch(t *testing.T) {
	// ApplyPatch: document with nil values, patch Value nil, nested null, array with nil
	t.Run("Apply_remove_key_whose_value_is_nil", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"foo": nil, "bar": "baz"}
		patches := []Patch{{Op: "remove", Path: "/foo"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		_, exists := result["foo"]
		assert.False(t, exists)
		assert.Equal(t, "baz", result["bar"])
	})
	t.Run("Apply_replace_nil_with_value", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"x": nil}
		patches := []Patch{{Op: "replace", Path: "/x", Value: "set"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "set", result["x"])
	})
	t.Run("Apply_add_nil_value_at_new_key", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"a": 1}
		patches := []Patch{{Op: "add", Path: "/nullkey", Value: nil}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Len(t, result, 2)
		_, hasKey := result["nullkey"]
		assert.True(t, hasKey)
		assert.Nil(t, result["nullkey"])
	})
	t.Run("Apply_add_overwrites_existing_nil", func(t *testing.T) {
		// Arrange - add at path that currently holds nil (object member exists)
		doc := map[string]any{"p": nil}
		patches := []Patch{{Op: "add", Path: "/p", Value: 42}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 42, result["p"])
	})
	t.Run("Apply_replace_at_array_index_where_element_is_nil", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, nil, 3}}
		patches := []Patch{{Op: "replace", Path: "/arr/1", Value: 2}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1, 2, 3}, result["arr"])
	})
	t.Run("Apply_remove_at_array_index_where_element_is_nil", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, nil, 3}}
		patches := []Patch{{Op: "remove", Path: "/arr/1"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1, 3}, result["arr"])
	})
	t.Run("Apply_move_from_path_whose_value_is_nil", func(t *testing.T) {
		// Arrange - move nil to another key
		doc := map[string]any{"from": nil, "to": "existing"}
		patches := []Patch{{Op: "move", From: "/from", Path: "/moved"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		_, fromExists := result["from"]
		assert.False(t, fromExists)
		assert.Nil(t, result["moved"])
		assert.Equal(t, "existing", result["to"])
	})
	t.Run("Apply_nested_object_with_nil_leaf", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"nested": map[string]any{"leaf": nil}}
		patches := []Patch{{Op: "replace", Path: "/nested/leaf", Value: "filled"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		nested := result["nested"].(map[string]any)
		assert.Equal(t, "filled", nested["leaf"])
	})
	t.Run("Apply_fails_when_traversing_through_nil", func(t *testing.T) {
		// Arrange - path /a/b where a is nil (cannot index into null)
		doc := map[string]any{"a": nil}
		patches := []Patch{{Op: "add", Path: "/a/b", Value: 1}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected type")
	})

	// GeneratePatch: before/after with nil, nil vs missing, round-trip with nil
	t.Run("Generate_add_when_after_has_nil_at_new_key", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a": 1}
		after := map[string]any{"a": 1, "n": nil}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Generate_remove_when_after_omits_key_that_was_nil", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a": 1, "n": nil}
		after := map[string]any{"a": 1}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Generate_no_patch_when_both_nil_at_same_key", func(t *testing.T) {
		// Arrange
		before := map[string]any{"x": nil}
		after := map[string]any{"x": nil}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, patches)
	})
	t.Run("Generate_replace_when_before_nil_after_non_nil", func(t *testing.T) {
		// Arrange
		before := map[string]any{"k": nil}
		after := map[string]any{"k": "v"}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Generate_round_trip_nested_nil", func(t *testing.T) {
		// Arrange
		before := map[string]any{"outer": map[string]any{"inner": nil}}
		after := map[string]any{"outer": map[string]any{"inner": 42}}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Generate_round_trip_array_with_nil_element_change", func(t *testing.T) {
		// Arrange - change one element in array that contains nil
		before := map[string]any{"arr": []any{nil, 2, nil}}
		after := map[string]any{"arr": []any{nil, 99, nil}}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
}

// --- Path and pointer edge cases (plan §4) ---

func TestShouldApplyPatchCorrectlyGivenPathAndPointerEdgeCasesWhenApplyingPatch(t *testing.T) {
	t.Run("Append_index_dash_succeeds", func(t *testing.T) {
		// Arrange - RFC 6902: /arr/- appends to array
		doc := map[string]any{"arr": []any{1, 2}}
		patches := []Patch{{Op: "add", Path: "/arr/-", Value: 3}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1, 2, 3}, result["arr"])
	})
	t.Run("Path_with_tilde1_literal_key", func(t *testing.T) {
		// Arrange - RFC 6901: key "~1" is referenced by path /~01 (~0 = ~, so ~01 = "~1")
		doc := map[string]any{"~1": 10}
		patches := []Patch{{Op: "replace", Path: "/~01", Value: 42}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 42, result["~1"])
	})
	t.Run("Path_with_tilde0_literal_key", func(t *testing.T) {
		// Arrange - RFC 6901: key "~0" is referenced by path /~00 (~0 = ~, so ~00 = "~0")
		doc := map[string]any{"~0": 9}
		patches := []Patch{{Op: "replace", Path: "/~00", Value: 99}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 99, result["~0"])
	})
	t.Run("Empty_path_segment_errors", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"foo": "bar"}
		patches := []Patch{{Op: "add", Path: "/foo//bar", Value: "x"}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty component")
	})
	t.Run("Non_numeric_array_index_errors", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, 2}}
		patches := []Patch{{Op: "remove", Path: "/arr/bar"}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
	})
	t.Run("Negative_array_index_errors", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, 2}}
		patches := []Patch{{Op: "add", Path: "/arr/-1", Value: 3}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "index")
	})
	t.Run("Float_array_index_errors", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, 2}}
		patches := []Patch{{Op: "remove", Path: "/arr/1.5"}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
	})
}

// --- Group A: Patch serialization falsy-value roundtrip (Bug 1) ---

func TestShouldPreserveFalsyValuesGivenPatchWithFalsyValueWhenMarshalAndUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{"Value_false", false},
		{"Value_zero", 0},
		{"Value_empty_string", ""},
		{"Value_nil", nil},
		{"Value_empty_array", []any{}},
		{"Value_empty_object", map[string]any{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			patch := []Patch{{Op: "replace", Path: "/key", Value: c.value}}

			// Act
			jsonBytes, err := json.Marshal(patch)
			require.NoError(t, err)
			var unmarshaled []Patch
			err = json.Unmarshal(jsonBytes, &unmarshaled)

			// Assert
			require.NoError(t, err)
			require.Len(t, unmarshaled, 1)
			// JSON decodes numbers as float64; 0 survives as float64(0)
			if c.name == "Value_zero" {
				require.NotNil(t, unmarshaled[0].Value)
				assert.Equal(t, float64(0), unmarshaled[0].Value)
			} else {
				assert.Equal(t, c.value, unmarshaled[0].Value, "falsy value should survive JSON roundtrip")
			}
		})
	}
}

// --- Group B: Move nil from array (Bug 2) ---

func TestShouldMoveNilElementGivenArrayWithNilWhenApplyingMovePatch(t *testing.T) {
	t.Run("Move_nil_from_root_array", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{nil, "keep"}}
		patches := []Patch{{Op: "move", From: "/arr/0", Path: "/moved"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Nil(t, result["moved"])
		assert.Equal(t, []any{"keep"}, result["arr"])
	})
	t.Run("Move_nil_from_nested_array", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"deep": map[string]any{"arr": []any{nil, 1, 2}}}
		patches := []Patch{{Op: "move", From: "/deep/arr/0", Path: "/target"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Nil(t, result["target"])
		assert.Equal(t, []any{1, 2}, result["deep"].(map[string]any)["arr"])
	})
}

// --- Group C: Whitespace-sensitive string comparison (Bug 3) ---

func TestShouldGenerateReplaceGivenWhitespaceOnlyChangeWhenGeneratingPatch(t *testing.T) {
	t.Run("Object_string_whitespace_change", func(t *testing.T) {
		// Arrange
		before := map[string]any{"s": " foo "}
		after := map[string]any{"s": "foo"}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, patches, "should produce replace for whitespace-only change")
		var found bool
		for _, p := range patches {
			if p.Op == "replace" && p.Path == "/s" {
				found = true
				assert.Equal(t, "foo", p.Value)
				break
			}
		}
		assert.True(t, found)
	})
	t.Run("Array_element_whitespace_change", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a": []any{" x "}}
		after := map[string]any{"a": []any{"x"}}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, patches)
		var found bool
		for _, p := range patches {
			if p.Op == "replace" && strings.HasPrefix(p.Path, "/a/") {
				found = true
				assert.Equal(t, "x", p.Value)
				break
			}
		}
		assert.True(t, found)
	})
}

// --- Group D: Struct omitempty handling (Bug 4) ---

func TestShouldRespectOmitemptyGivenStructWithZeroFieldsWhenGeneratingPatch(t *testing.T) {
	type WithOmitempty struct {
		Required string `json:"required"`
		Optional string `json:"opt,omitempty"`
	}
	t.Run("No_patch_when_both_zero_optional", func(t *testing.T) {
		// Arrange
		before := WithOmitempty{Required: "a", Optional: ""}
		after := WithOmitempty{Required: "a", Optional: ""}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, patches, "omitempty zero in both should not produce patch")
	})
	t.Run("Patch_when_optional_goes_from_zero_to_set", func(t *testing.T) {
		// Arrange
		before := WithOmitempty{Required: "a", Optional: ""}
		after := WithOmitempty{Required: "a", Optional: "set"}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, patches)
		var found bool
		for _, p := range patches {
			if (p.Op == "add" || p.Op == "replace") && (p.Path == "/opt" || strings.Contains(p.Path, "opt")) {
				found = true
				assert.Equal(t, "set", p.Value)
				break
			}
		}
		assert.True(t, found)
	})
}

// --- Group E: JSON Pointer escaping (Bug 5) ---

func TestShouldHandleKeysWithSlashAndTildeGivenEscapedPathWhenApplyingPatch(t *testing.T) {
	t.Run("Key_contains_slash_escaped_as_tilde1", func(t *testing.T) {
		// Arrange - RFC 6901: ~1 encodes /
		doc := map[string]any{"a/b": 1}
		patches := []Patch{{Op: "replace", Path: "/a~1b", Value: 2}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, result["a/b"])
	})
	t.Run("Key_contains_tilde_escaped_as_tilde0", func(t *testing.T) {
		// Arrange - RFC 6901: ~0 encodes ~
		doc := map[string]any{"a~b": 1}
		patches := []Patch{{Op: "replace", Path: "/a~0b", Value: 2}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, result["a~b"])
	})
	t.Run("Key_tilde1_per_RFC_A14", func(t *testing.T) {
		// Arrange - path /~01 means key "~1" (segment ~01 unescapes to ~1)
		doc := map[string]any{"~1": 10}
		patches := []Patch{{Op: "replace", Path: "/~01", Value: 42}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 42, result["~1"])
	})
}

// --- Group F: Append index dash (Bug 6) ---

func TestShouldAppendToArrayGivenDashIndexWhenApplyingAddPatch(t *testing.T) {
	t.Run("Append_to_non_empty_array", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, 2}}
		patches := []Patch{{Op: "add", Path: "/arr/-", Value: 3}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1, 2, 3}, result["arr"])
	})
	t.Run("Append_to_empty_array", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{}}
		patches := []Patch{{Op: "add", Path: "/arr/-", Value: "first"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{"first"}, result["arr"])
	})
	t.Run("Append_to_nested_array", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"obj": map[string]any{"arr": []any{1}}}
		patches := []Patch{{Op: "add", Path: "/obj/arr/-", Value: 2}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1, 2}, result["obj"].(map[string]any)["arr"])
	})
}

// --- Group G: Misc edge cases ---

func TestShouldHandleMiscEdgeCasesGivenVariousInputsWhenApplyingOrGeneratingPatch(t *testing.T) {
	t.Run("Add_replaces_existing_key", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"foo": "old"}
		patches := []Patch{{Op: "add", Path: "/foo", Value: "new"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "new", result["foo"])
	})
	t.Run("Replace_with_identical_value", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"x": 1}
		patches := []Patch{{Op: "replace", Path: "/x", Value: 1}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, result["x"])
	})
	t.Run("Multiple_patches_same_path", func(t *testing.T) {
		// Arrange
		doc := map[string]any{}
		patches := []Patch{
			{Op: "add", Path: "/foo", Value: 1},
			{Op: "replace", Path: "/foo", Value: 2},
		}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, result["foo"])
	})
	t.Run("Empty_arrays_no_patches", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a": []any{}}
		after := map[string]any{"a": []any{}}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, patches)
	})
	t.Run("Array_to_empty_removes", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a": []any{1, 2}}
		after := map[string]any{"a": []any{}}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Empty_to_array_adds", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a": []any{}}
		after := map[string]any{"a": []any{1, 2}}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Key_with_spaces", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"hello world": 1}
		patches := []Patch{{Op: "replace", Path: "/hello world", Value: 2}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, result["hello world"])
	})
	t.Run("Numeric_string_key_treated_as_object_member", func(t *testing.T) {
		// Arrange - "0" is object key, not array index
		doc := map[string]any{"0": "val"}
		patches := []Patch{{Op: "replace", Path: "/0", Value: "new"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "new", result["0"])
	})
	t.Run("Struct_with_pointer_field", func(t *testing.T) {
		type PtrStruct struct {
			Name *string `json:"name"`
		}
		s := "alice"
		before := PtrStruct{Name: &s}
		after := PtrStruct{Name: nil}

		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(map[string]any{"name": "alice"}, patches)
		require.NoError(t, err)
		assert.Nil(t, result["name"])
	})
	t.Run("Deeply_nested_array_of_objects_roundtrip", func(t *testing.T) {
		// Arrange
		before := map[string]any{
			"level1": []any{
				map[string]any{"level2": []any{
					map[string]any{"k": "a"},
				}},
			},
		}
		after := map[string]any{
			"level1": []any{
				map[string]any{"level2": []any{
					map[string]any{"k": "b"},
				}},
			},
		}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
}

// --- Group H: test operation (Bug 7) ---

func TestShouldSucceedGivenMatchingValueWhenApplyingTestPatch(t *testing.T) {
	t.Run("String_value_matches", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"foo": "bar"}
		patches := []Patch{{Op: "test", Path: "/foo", Value: "bar"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "bar", result["foo"])
	})
	t.Run("String_value_mismatch_errors", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"foo": "bar"}
		patches := []Patch{{Op: "test", Path: "/foo", Value: "baz"}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test failed")
	})
	t.Run("Nested_value_matches", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"a": map[string]any{"b": 1}}
		patches := []Patch{{Op: "test", Path: "/a/b", Value: 1}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, result["a"].(map[string]any)["b"])
	})
	t.Run("Array_element_matches", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, 2, 3}}
		patches := []Patch{{Op: "test", Path: "/arr/1", Value: 2}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1, 2, 3}, result["arr"])
	})
	t.Run("Numeric_value_matches_across_json_types", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1}}
		patches := []Patch{{Op: "test", Path: "/arr/0", Value: 1.0}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1}, result["arr"])
	})
	t.Run("Null_value_matches", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"n": nil}
		patches := []Patch{{Op: "test", Path: "/n", Value: nil}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Nil(t, result["n"])
	})
	t.Run("Test_then_replace_succeeds", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"x": 1}
		patches := []Patch{
			{Op: "test", Path: "/x", Value: 1},
			{Op: "replace", Path: "/x", Value: 2},
		}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 2, result["x"])
	})
	t.Run("Test_failure_aborts_subsequent_ops", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"x": 1}
		patches := []Patch{
			{Op: "test", Path: "/x", Value: 99},
			{Op: "replace", Path: "/x", Value: 2},
		}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
	})
}

// --- Group I: copy operation (Bug 8) ---

func TestShouldCopyValueGivenValidFromPathWhenApplyingCopyPatch(t *testing.T) {
	t.Run("Copy_string_value", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"foo": "bar"}
		patches := []Patch{{Op: "copy", From: "/foo", Path: "/baz"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "bar", result["foo"])
		assert.Equal(t, "bar", result["baz"])
	})
	t.Run("Copy_deep_copies_object", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"a": map[string]any{"b": 1}}
		patches := []Patch{{Op: "copy", From: "/a", Path: "/c"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"b": 1}, result["a"])
		assert.Equal(t, map[string]any{"b": 1}, result["c"])
		// Mutating the copy should not affect the original
		result["c"].(map[string]any)["b"] = 999
		assert.Equal(t, 1, result["a"].(map[string]any)["b"])
	})
	t.Run("Copy_array_element", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"arr": []any{1, 2}}
		patches := []Patch{{Op: "copy", From: "/arr/0", Path: "/first"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, []any{1, 2}, result["arr"])
		assert.Equal(t, 1, result["first"])
	})
	t.Run("Copy_from_nonexistent_errors", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"foo": "bar"}
		patches := []Patch{{Op: "copy", From: "/nope", Path: "/baz"}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
	})
}

// --- Group J: Move from-prefix-of-path (Bug 9) ---

func TestShouldErrorGivenFromIsPrefixOfPathWhenApplyingMovePatch(t *testing.T) {
	t.Run("From_is_proper_prefix_of_path", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"a": map[string]any{"b": 1}}
		patches := []Patch{{Op: "move", From: "/a", Path: "/a/b"}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prefix")
	})
	t.Run("Deeper_prefix", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"a": map[string]any{"b": map[string]any{"c": 1}}}
		patches := []Patch{{Op: "move", From: "/a/b", Path: "/a/b/c"}}

		// Act
		_, err := ApplyPatch(doc, patches)

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prefix")
	})
	t.Run("Not_a_prefix_succeeds", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"a": 1, "b": 2}
		patches := []Patch{{Op: "move", From: "/a", Path: "/b"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, result["b"])
		_, exists := result["a"]
		assert.False(t, exists)
	})
	t.Run("Shared_parent_not_prefix_succeeds", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"a": map[string]any{"b": 1, "c": 2}}
		patches := []Patch{{Op: "move", From: "/a/b", Path: "/a/c"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, 1, result["a"].(map[string]any)["c"])
	})
}

// --- Group K: toMap nil pointer (Bug 10) ---

func TestShouldNotPanicGivenNilPointerToStructWhenGeneratingPatch(t *testing.T) {
	type SimpleStruct struct {
		Name string `json:"name"`
	}
	t.Run("Both_nil_pointers", func(t *testing.T) {
		// Arrange
		var before *SimpleStruct
		var after *SimpleStruct

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, patches)
	})
	t.Run("Nil_before_non_nil_after", func(t *testing.T) {
		// Arrange
		var before *SimpleStruct
		after := &SimpleStruct{Name: "x"}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		require.NotEmpty(t, patches)
	})
	t.Run("ApplyPatch_on_nil_pointer", func(t *testing.T) {
		// Arrange
		var s *SimpleStruct
		patches := []Patch{{Op: "add", Path: "/name", Value: "y"}}

		// Act
		result, err := ApplyPatch(s, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "y", result["name"])
	})
}

// --- Group L: Embedded struct field promotion (Bug 11) ---

func TestShouldPromoteEmbeddedFieldsGivenStructWithAnonymousFieldWhenGeneratingPatch(t *testing.T) {
	type EmbedBase struct {
		ID string `json:"id"`
	}
	type EmbedExtended struct {
		EmbedBase

		Name string `json:"name"`
	}
	t.Run("Embedded_fields_promoted_no_phantom_diff", func(t *testing.T) {
		// Arrange
		before := EmbedExtended{EmbedBase: EmbedBase{ID: "1"}, Name: "a"}
		after := EmbedExtended{EmbedBase: EmbedBase{ID: "1"}, Name: "b"}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		require.Len(t, patches, 1)
		assert.Equal(t, "replace", patches[0].Op)
		assert.Equal(t, "/name", patches[0].Path)
		assert.Equal(t, "b", patches[0].Value)
	})
	t.Run("Embedded_field_change_detected", func(t *testing.T) {
		// Arrange
		before := EmbedExtended{EmbedBase: EmbedBase{ID: "1"}, Name: "a"}
		after := EmbedExtended{EmbedBase: EmbedBase{ID: "2"}, Name: "a"}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		require.Len(t, patches, 1)
		assert.Equal(t, "replace", patches[0].Op)
		assert.Equal(t, "/id", patches[0].Path)
	})
	t.Run("No_diff_when_identical", func(t *testing.T) {
		// Arrange
		before := EmbedExtended{EmbedBase: EmbedBase{ID: "1"}, Name: "a"}
		after := EmbedExtended{EmbedBase: EmbedBase{ID: "1"}, Name: "a"}

		// Act
		patches, err := GeneratePatch(before, after, "")

		// Assert
		require.NoError(t, err)
		assert.Empty(t, patches)
	})
}

// --- Group M: Escaped key generate+apply roundtrip ---

func TestShouldRoundtripPatchGivenKeysWithSlashOrTildeWhenGeneratingAndApplying(t *testing.T) {
	t.Run("Key_with_slash_roundtrip", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a/b": 1}
		after := map[string]any{"a/b": 2}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Key_with_tilde_roundtrip", func(t *testing.T) {
		// Arrange
		before := map[string]any{"~x": 1}
		after := map[string]any{"~x": 2}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Removal_of_escaped_keys_roundtrip", func(t *testing.T) {
		// Arrange
		before := map[string]any{"a/b": 1, "~x": 2}
		after := map[string]any{}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
}

// --- Group N: Misc remaining edge cases ---

func TestShouldHandleRemainingEdgeCasesGivenVariousInputsWhenApplyingOrGeneratingPatch(t *testing.T) {
	t.Run("Empty_patch_list_returns_deep_copy", func(t *testing.T) {
		// Arrange
		doc := map[string]any{"k": "v"}

		// Act
		result, err := ApplyPatch(doc, []Patch{})

		// Assert
		require.NoError(t, err)
		assert.Equal(t, doc, result)
		// Mutating result should not affect original
		result["k"] = "changed"
		assert.Equal(t, "v", doc["k"])
	})
	t.Run("Nil_to_value_roundtrip", func(t *testing.T) {
		// Arrange
		before := map[string]any{"k": nil}
		after := map[string]any{"k": "set"}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Value_to_nil_roundtrip", func(t *testing.T) {
		// Arrange
		before := map[string]any{"k": "set"}
		after := map[string]any{"k": nil}

		// Act
		patches, err := GeneratePatch(before, after, "")
		require.NoError(t, err)
		result, err := ApplyPatch(before, patches)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, after, result)
	})
	t.Run("Move_within_same_array", func(t *testing.T) {
		// Arrange - RFC 6902: move is remove-then-add
		doc := map[string]any{"arr": []any{"a", "b", "c"}}
		patches := []Patch{{Op: "move", From: "/arr/0", Path: "/arr/2"}}

		// Act
		result, err := ApplyPatch(doc, patches)

		// Assert - remove index 0 → ["b","c"], then add at index 2 → ["b","c","a"]
		require.NoError(t, err)
		assert.Equal(t, []any{"b", "c", "a"}, result["arr"])
	})
}
