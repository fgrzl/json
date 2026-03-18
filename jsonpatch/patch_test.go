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

func TestShouldReturnErrorWhenApplyingPatchWithEmptyPath(t *testing.T) {
	// Arrange
	original := map[string]any{"key": "value"}
	patches := []Patch{{Op: "add", Path: "", Value: "value"}} // Empty path

	// Act
	result, err := ApplyPatch(original, patches)

	// Assert
	assert.Error(t, err, "Should error on empty path")
	assert.Nil(t, result, "Result should be nil on error")
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
	assert.ErrorContains(t, err, "invalid index -1")
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
	assert.ErrorContains(t, err, "invalid index 5")
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
