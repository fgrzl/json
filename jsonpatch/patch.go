// Package jsonpatch provides functionality to generate and apply JSON Patch operations.
//
// The package includes functions to generate a JSON patch that describes the differences
// between two JSON documents and apply a series of patch operations to a JSON document.
//
// Types:
// - Patch: Represents a single JSON Patch operation.
//
// Functions:
// - GeneratePatch: Compares two documents and returns patch operations.
// - ApplyPatch: Applies a series of patch operations to a JSON document.
//
// Helper Functions:
// - toMap: Converts a struct (or already a map) to a map[string]interface{} without JSON round-trips.
// - toSlice: Converts an array/slice to []interface{} using reflection.
// - deepEqualFiltered: Compares two values; for strings it trims whitespace.
// - generateArrayPatch: Generates patch operations to transform one array into another.
// - parsePath: Splits a JSON pointer path into its components.
// - isTwoPartArray: Checks if the path has exactly two parts and that the first part refers to an array.
// - traverseToParent: Walks the target object to the parent of the final key.
// - applyAdd: Applies an "add" operation.
// - insertIntoSlice: Inserts a value into a slice at the given index.
// - applyRemove: Applies a "remove" operation.
// - removeFromSlice: Removes an element from a slice.
// - applyReplace: Applies a "replace" operation.
// - replaceInSlice: Replaces an element in a slice.
// - applyMove: Applies a "move" operation.
// - getFromSlice: Retrieves a value from a slice.
package jsonpatch

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Patch represents a single JSON Patch operation.
type Patch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	From  string      `json:"from,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// GeneratePatch compares two documents and returns patch operations.
// GeneratePatch generates a JSON patch that describes the differences between
// the "before" and "after" JSON documents. The function returns a slice of Patch
// objects that represent the changes needed to transform the "before" document
// into the "after" document.
//
// Parameters:
// - before: The original JSON document, represented as an interface{}.
// - after: The modified JSON document, represented as an interface{}.
// - basePath: The base path for the JSON patch operations.
//
// Returns:
//   - A slice of Patch objects representing the differences between the "before"
//     and "after" documents.
//   - An error if any issues occur during the patch generation process.
//
// The function processes keys in the "after" document to identify additions,
// replacements, and nested changes. It also processes keys in the "before"
// document to identify removals.
func GeneratePatch(before, after interface{}, basePath string) ([]Patch, error) {
	patches := []Patch{}
	beforeMap, err := toMap(before)
	if err != nil {
		return nil, err
	}
	afterMap, err := toMap(after)
	if err != nil {
		return nil, err
	}

	// Process keys in the "after" document.
	for key, afterVal := range afterMap {
		path := basePath + "/" + key
		beforeVal, exists := beforeMap[key]
		if !exists {
			patches = append(patches, Patch{Op: "add", Path: path, Value: afterVal})
			continue
		}
		// If types differ, emit a replace.
		if reflect.TypeOf(beforeVal) != reflect.TypeOf(afterVal) {
			patches = append(patches, Patch{Op: "replace", Path: path, Value: afterVal})
			continue
		}
		switch reflect.TypeOf(beforeVal).Kind() {
		case reflect.Slice:
			arrOps, err := generateArrayPatch(path, beforeVal, afterVal)
			if err != nil {
				return nil, err
			}
			patches = append(patches, arrOps...)
		case reflect.Map, reflect.Struct:
			nested, err := GeneratePatch(beforeVal, afterVal, path)
			if err != nil {
				return nil, err
			}
			patches = append(patches, nested...)
		default:
			if !deepEqualFiltered(beforeVal, afterVal) {
				patches = append(patches, Patch{Op: "replace", Path: path, Value: afterVal})
			}
		}
	}

	// Process removals.
	for key := range beforeMap {
		if _, exists := afterMap[key]; !exists {
			patches = append(patches, Patch{Op: "remove", Path: basePath + "/" + key})
		}
	}
	return patches, nil
}

// ApplyPatch applies a series of JSON Patch operations to an original JSON object.
// It supports the following operations: add, remove, replace, and move.
//
// Parameters:
// - original: The original JSON object to which the patches will be applied.
// - patches: A slice of Patch objects representing the operations to be applied.
//
// Returns:
// - An updated JSON object with the patches applied.
// - An error if any of the operations fail.
//
// Example usage:
//
//	patched, err := ApplyPatch(original, patches)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Note: The function assumes that the original JSON object is represented as an interface{} and
//
//	the patches are provided as a slice of Patch objects.
func ApplyPatch(original interface{}, patches []Patch) (interface{}, error) {
	target, err := toMap(original)
	if err != nil {
		return nil, err
	}
	for _, op := range patches {
		parts, err := parsePath(op.Path)
		if err != nil {
			return nil, err
		}
		switch op.Op {
		case "add":
			err = applyAdd(target, parts, op.Value)
		case "remove":
			err = applyRemove(target, parts)
		case "replace":
			err = applyReplace(target, parts, op.Value)
		case "move":
			fromParts, err := parsePath(op.From)
			if err != nil {
				return nil, err
			}
			err = applyMove(target, fromParts, parts)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported op: %s", op.Op)
		}
		if err != nil {
			return nil, err
		}
	}
	return target, nil
}

// toMap converts a struct (or already a map) to a map[string]interface{} without JSON round-trips.
func toMap(data interface{}) (map[string]interface{}, error) {
	if m, ok := data.(map[string]interface{}); ok {
		return m, nil
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported type %T for conversion to map", data)
	}
	result := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		key := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] != "" && parts[0] != "-" {
				key = parts[0]
			}
		}
		result[key] = v.Field(i).Interface()
	}
	return result, nil
}

// toSlice converts an array/slice to []interface{} using reflection.
func toSlice(data interface{}) ([]interface{}, error) {
	if s, ok := data.([]interface{}); ok {
		return s, nil
	}
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil, fmt.Errorf("unsupported type %T for conversion to slice", data)
	}
	result := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		result[i] = v.Index(i).Interface()
	}
	return result, nil
}

// deepEqualFiltered compares two values; for strings it trims whitespace.
func deepEqualFiltered(a, b interface{}) bool {
	aStr, aOk := a.(string)
	bStr, bOk := b.(string)
	if aOk && bOk {
		return strings.TrimSpace(aStr) == strings.TrimSpace(bStr)
	}
	return reflect.DeepEqual(a, b)
}

// generateArrayPatch generates patch operations to transform one array into another.
func generateArrayPatch(basePath string, before, after interface{}) ([]Patch, error) {
	beforeSlice, err := toSlice(before)
	if err != nil {
		return nil, err
	}
	afterSlice, err := toSlice(after)
	if err != nil {
		return nil, err
	}

	// Detect a simple swap.
	if len(beforeSlice) == len(afterSlice) {
		var diffIndices []int
		for i := 0; i < len(beforeSlice); i++ {
			if !deepEqualFiltered(beforeSlice[i], afterSlice[i]) {
				diffIndices = append(diffIndices, i)
			}
		}
		if len(diffIndices) == 2 {
			i, j := diffIndices[0], diffIndices[1]
			if deepEqualFiltered(beforeSlice[i], afterSlice[j]) && deepEqualFiltered(beforeSlice[j], afterSlice[i]) {
				return []Patch{
					{Op: "move", Path: basePath + "/" + strconv.Itoa(j), From: basePath + "/" + strconv.Itoa(i)},
				}, nil
			}
		}
	}

	// Fallback: element-by-element diffing.
	var patches []Patch
	commonLen := len(beforeSlice)
	if len(afterSlice) < commonLen {
		commonLen = len(afterSlice)
	}
	for i := 0; i < commonLen; i++ {
		if !deepEqualFiltered(beforeSlice[i], afterSlice[i]) {
			patches = append(patches, Patch{
				Op:    "replace",
				Path:  basePath + "/" + strconv.Itoa(i),
				Value: afterSlice[i],
			})
		}
	}
	for i := commonLen; i < len(afterSlice); i++ {
		patches = append(patches, Patch{
			Op:    "add",
			Path:  basePath + "/" + strconv.Itoa(i),
			Value: afterSlice[i],
		})
	}
	for i := len(afterSlice); i < len(beforeSlice); i++ {
		patches = append(patches, Patch{
			Op:   "remove",
			Path: basePath + "/" + strconv.Itoa(i),
		})
	}
	return patches, nil
}

// parsePath splits a JSON pointer path into its components.
func parsePath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	return strings.Split(strings.Trim(path, "/"), "/"), nil
}

// isTwoPartArray checks if the path has exactly two parts and that the first part refers to an array.
func isTwoPartArray(target map[string]interface{}, parts []string) (string, int, error) {
	if len(parts) != 2 {
		return "", -1, fmt.Errorf("expected 2 parts, got %d", len(parts))
	}
	key := parts[0]
	arr, ok := target[key].([]interface{})
	if !ok {
		return "", -1, fmt.Errorf("target[%s] is not an array", key)
	}
	idx, err := strconv.Atoi(parts[1])
	if err != nil || idx < 0 || idx >= len(arr) {
		return "", -1, fmt.Errorf("invalid index %s", parts[1])
	}
	return key, idx, nil
}

// traverseToParent walks the target object to the parent of the final key.
func traverseToParent(target map[string]interface{}, parts []string) (map[string]interface{}, string, bool, int, error) {
	parent := target
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		val, exists := parent[part]
		if !exists {
			return nil, "", false, -1, fmt.Errorf("path %s does not exist", strings.Join(parts[:i+1], "/"))
		}
		if i == len(parts)-2 {
			// If the value is an array, treat the next part as an index.
			if arr, ok := val.([]interface{}); ok {
				j, err := strconv.Atoi(parts[i+1])
				if err != nil || j < 0 || j >= len(arr) {
					return nil, "", false, -1, fmt.Errorf("invalid index %s", parts[i+1])
				}
				return parent, part, true, j, nil
			}
			// Otherwise, assume it's a map.
			if m, ok := val.(map[string]interface{}); ok {
				parent = m
				return parent, parts[len(parts)-1], false, -1, nil
			}
			return nil, "", false, -1, fmt.Errorf("unexpected type at %s", part)
		} else {
			if m, ok := val.(map[string]interface{}); ok {
				parent = m
			} else {
				return nil, "", false, -1, fmt.Errorf("expected map at %s", part)
			}
		}
	}
	return parent, parts[len(parts)-1], false, -1, nil
}

// applyAdd applies an "add" operation.
func applyAdd(target map[string]interface{}, parts []string, value interface{}) error {
	// Special handling: if two parts and target[parts[0]] is an array and index equals len.
	if len(parts) == 2 {
		if arr, ok := target[parts[0]].([]interface{}); ok {
			idx, err := strconv.Atoi(parts[1])
			if err == nil && idx == len(arr) {
				target[parts[0]] = append(arr, value)
				return nil
			}
		}
	}
	// Try direct two-part array handling.
	if key, idx, err := isTwoPartArray(target, parts); err == nil {
		arr := target[key].([]interface{})
		if idx == len(arr) {
			target[key] = append(arr, value)
		} else {
			target[key] = append(arr[:idx], append([]interface{}{value}, arr[idx:]...)...)
		}
		return nil
	}
	parent, key, isArr, idx, err := traverseToParent(target, parts)
	if err != nil {
		return err
	}
	if isArr {
		return insertIntoSlice(parent, key, idx, value)
	}
	parent[key] = value
	return nil
}

// insertIntoSlice inserts a value into a slice at the given index.
func insertIntoSlice(parent map[string]interface{}, key string, index int, value interface{}) error {
	arr, ok := parent[key].([]interface{})
	if !ok {
		parent[key] = []interface{}{value}
		return nil
	}
	if index < 0 || index > len(arr) {
		return fmt.Errorf("index %d out of bounds", index)
	}
	if index == len(arr) {
		parent[key] = append(arr, value)
	} else {
		parent[key] = append(arr[:index], append([]interface{}{value}, arr[index:]...)...)
	}
	return nil
}

// applyRemove applies a "remove" operation.
func applyRemove(target map[string]interface{}, parts []string) error {
	if key, idx, err := isTwoPartArray(target, parts); err == nil {
		arr := target[key].([]interface{})
		if idx < 0 || idx >= len(arr) {
			return fmt.Errorf("index %d out of bounds", idx)
		}
		target[key] = append(arr[:idx], arr[idx+1:]...)
		return nil
	}
	parent, key, isArr, idx, err := traverseToParent(target, parts)
	if err != nil {
		return err
	}
	if isArr {
		return removeFromSlice(parent, key, idx)
	}
	delete(parent, key)
	return nil
}

// removeFromSlice removes an element from a slice.
func removeFromSlice(parent map[string]interface{}, key string, index int) error {
	arr, ok := parent[key].([]interface{})
	if !ok || index < 0 || index >= len(arr) {
		return fmt.Errorf("invalid array or index %d", index)
	}
	parent[key] = append(arr[:index], arr[index+1:]...)
	return nil
}

// applyReplace applies a "replace" operation.
func applyReplace(target map[string]interface{}, parts []string, value interface{}) error {
	if key, idx, err := isTwoPartArray(target, parts); err == nil {
		arr := target[key].([]interface{})
		if idx < 0 || idx >= len(arr) {
			return fmt.Errorf("index %d out of bounds", idx)
		}
		arr[idx] = value
		target[key] = arr
		return nil
	}
	parent, key, isArr, idx, err := traverseToParent(target, parts)
	if err != nil {
		return err
	}
	if isArr {
		return replaceInSlice(parent, key, idx, value)
	}
	parent[key] = value
	return nil
}

// replaceInSlice replaces an element in a slice.
func replaceInSlice(parent map[string]interface{}, key string, index int, value interface{}) error {
	arr, ok := parent[key].([]interface{})
	if !ok || index < 0 || index >= len(arr) {
		return fmt.Errorf("invalid array or index %d", index)
	}
	arr[index] = value
	parent[key] = arr
	return nil
}

// applyMove applies a "move" operation.
func applyMove(target map[string]interface{}, fromParts, toParts []string) error {
	var value interface{}
	if key, idx, err := isTwoPartArray(target, fromParts); err == nil {
		arr := target[key].([]interface{})
		value = arr[idx]
	} else {
		parent, key, isArr, idx, err := traverseToParent(target, fromParts)
		if err != nil {
			return err
		}
		if isArr {
			value = getFromSlice(parent, key, idx)
		} else {
			value = parent[key]
		}
	}
	if err := applyRemove(target, fromParts); err != nil {
		return err
	}
	return applyAdd(target, toParts, value)
}

// getFromSlice retrieves a value from a slice.
func getFromSlice(parent map[string]interface{}, key string, index int) interface{} {
	arr, ok := parent[key].([]interface{})
	if !ok || index < 0 || index >= len(arr) {
		return nil
	}
	return arr[index]
}
