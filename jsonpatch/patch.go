// Package jsonpatch provides utilities for generating and applying JSON Patch operations.
//
// The package includes functions to generate JSON Patch operations by comparing two JSON documents,
// and to apply a list of patch operations to a JSON document.
//
// Types:
// - Patch: Represents a single JSON Patch operation.
//
// Functions:
// - GeneratePatch: Compares two documents and returns patch operations.
// - ApplyPatch: Applies a list of patch operations to the original document.
//
// Internal Functions:
// - generateArrayPatch: Generates JSON Patch operations for arrays.
// - deepEqualFiltered: Compares two values with special handling for strings.
// - toSlice: Converts data to []interface{} via JSON round-trip.
// - structToMap: Converts data to map[string]interface{} via JSON round-trip.
//
// Helper Functions for ApplyPatch:
// - isTwoPartArray: Checks if parts has exactly two elements and target[parts[0]] is a slice.
// - traverseToParent: Walks through target for paths longer than 2 parts.
// - applyAdd: Applies an "add" operation to the target.
// - applyRemove: Applies a "remove" operation to the target.
// - applyReplace: Applies a "replace" operation to the target.
// - applyMove: Applies a "move" operation to the target.
// - insertIntoSlice: Inserts a value into a slice at a specified index.
// - removeFromSlice: Removes a value from a slice at a specified index.
// - replaceInSlice: Replaces a value in a slice at a specified index.
// - getFromSlice: Retrieves a value from a slice at a specified index.
package jsonpatch

import (
	"encoding/json"
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
// For arrays of equal length, differing elements yield a single "replace" op.
func GeneratePatch(before, after interface{}, basePath string) ([]Patch, error) {
	var patches []Patch

	beforeMap, err := structToMap(before)
	if err != nil {
		return nil, err
	}
	afterMap, err := structToMap(after)
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
		switch reflect.TypeOf(beforeVal).Kind() {
		case reflect.Slice:
			arrOps, err := generateArrayPatch(path, beforeVal, afterVal)
			if err != nil {
				return nil, err
			}
			patches = append(patches, arrOps...)
		case reflect.Map:
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

// generateArrayPatch compares two arrays element-by-element.
// When arrays are equal in length, it tries to detect a simple swap
// that can be expressed as a "move" op.
// generateArrayPatch generates a JSON Patch array operation list to transform
// the 'before' array into the 'after' array. It detects simple swaps and emits
// a move operation if applicable, otherwise it generates replace, add, and remove
// operations as needed.
//
// Parameters:
//   - basePath: The base path for the JSON Patch operations.
//   - before: The original array.
//   - after: The modified array.
//
// Returns:
//   - A slice of Patch operations to transform the 'before' array into the 'after' array.
//   - An error if the input arrays cannot be converted to slices or if any other error occurs.
func generateArrayPatch(basePath string, before, after interface{}) ([]Patch, error) {
	beforeSlice, err := toSlice(before)
	if err != nil {
		return nil, err
	}
	afterSlice, err := toSlice(after)
	if err != nil {
		return nil, err
	}

	// Detect a simple swap: if arrays are equal in length and exactly two indices differ,
	// and they are swapped, emit a move op.
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
					{
						Op:   "move",
						Path: basePath + "/" + strconv.Itoa(j),
						From: basePath + "/" + strconv.Itoa(i),
					},
				}, nil
			}
		}
	}

	// Fallback: generate replace, add, and remove ops element-by-element.
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

// deepEqualFiltered compares two values; for strings it trims whitespace.
func deepEqualFiltered(a, b interface{}) bool {
	aStr, aOk := a.(string)
	bStr, bOk := b.(string)
	if aOk && bOk {
		return strings.TrimSpace(aStr) == strings.TrimSpace(bStr)
	}
	return reflect.DeepEqual(a, b)
}

// toSlice converts data to []interface{} via JSON round-trip.
func toSlice(data interface{}) ([]interface{}, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var result []interface{}
	err = json.Unmarshal(j, &result)
	return result, err
}

// structToMap converts data to map[string]interface{} via JSON round-trip.
func structToMap(data interface{}) (map[string]interface{}, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal(j, &result)
	return result, err
}

// ApplyPatch applies a list of patch operations to the original document.
func ApplyPatch(original interface{}, patches []Patch) (interface{}, error) {
	target, err := structToMap(original)
	if err != nil {
		return nil, err
	}
	for _, op := range patches {
		parts := strings.Split(strings.Trim(op.Path, "/"), "/")
		switch op.Op {
		case "add":
			applyAdd(target, parts, op.Value)
		case "remove":
			applyRemove(target, parts)
		case "replace":
			applyReplace(target, parts, op.Value)
		case "move":
			applyMove(target, op.From, op.Path)
		default:
			return nil, fmt.Errorf("unsupported op: %s", op.Op)
		}
	}
	return target, nil
}

// --- Apply Patch Helpers ---

// isTwoPartArray returns (key, idx, true) if parts has exactly two elements and target[parts[0]] is a slice.
func isTwoPartArray(target map[string]interface{}, parts []string) (string, int, bool) {
	if len(parts) != 2 {
		return "", -1, false
	}
	key := parts[0]
	arr, exists := target[key].([]interface{})
	if !exists {
		return "", -1, false
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil || i < 0 || i >= len(arr) {
		return "", -1, false
	}
	return key, i, true
}

// traverseToParent walks through target for paths longer than 2 parts.
// It returns the parent map, the final key (or array key), and if that container is an array then isArr==true with idx set.
func traverseToParent(target map[string]interface{}, parts []string) (map[string]interface{}, string, bool, int) {
	parent := target
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		val, exists := parent[part]
		if !exists {
			return nil, "", false, -1
		}
		if i == len(parts)-2 {
			// If the value is an array, then treat parts[i+1] as index.
			if arr, ok := val.([]interface{}); ok {
				j, err := strconv.Atoi(parts[i+1])
				if err != nil || j < 0 || j >= len(arr) {
					return nil, "", false, -1
				}
				return parent, part, true, j
			}
			// Otherwise, assume it's a map.
			if m, ok := val.(map[string]interface{}); ok {
				parent = m
				return parent, parts[len(parts)-1], false, -1
			}
			return nil, "", false, -1
		} else {
			if m, ok := val.(map[string]interface{}); ok {
				parent = m
			} else {
				return nil, "", false, -1
			}
		}
	}
	return parent, parts[len(parts)-1], false, -1
}

func applyAdd(target map[string]interface{}, parts []string, value interface{}) {
	// Special handling for adding to the end of an array.
	if len(parts) == 2 {
		key := parts[0]
		if arr, ok := target[key].([]interface{}); ok {
			idx, err := strconv.Atoi(parts[1])
			if err == nil && idx == len(arr) {
				target[key] = append(arr, value)
				return
			}
		}
	}
	// Direct handling for two-part array paths.
	if key, i, ok := isTwoPartArray(target, parts); ok {
		arr := target[key].([]interface{})
		if i == len(arr) {
			target[key] = append(arr, value)
		} else {
			target[key] = append(arr[:i], append([]interface{}{value}, arr[i:]...)...)
		}
		return
	}
	parent, key, isArr, i := traverseToParent(target, parts)
	if parent == nil {
		return
	}
	if isArr {
		insertIntoSlice(parent, key, i, value)
	} else {
		parent[key] = value
	}
}

func applyRemove(target map[string]interface{}, parts []string) {
	if key, i, ok := isTwoPartArray(target, parts); ok {
		arr := target[key].([]interface{})
		target[key] = append(arr[:i], arr[i+1:]...)
		return
	}
	parent, key, isArr, i := traverseToParent(target, parts)
	if parent == nil {
		return
	}
	if isArr {
		removeFromSlice(parent, key, i)
	} else {
		delete(parent, key)
	}
}

func applyReplace(target map[string]interface{}, parts []string, value interface{}) {
	// Direct handling for two-part array paths.
	if key, i, ok := isTwoPartArray(target, parts); ok {
		arr := target[key].([]interface{})
		arr[i] = value
		target[key] = arr
		return
	}
	parent, key, isArr, i := traverseToParent(target, parts)
	if parent == nil {
		return
	}
	if isArr {
		replaceInSlice(parent, key, i, value)
	} else {
		parent[key] = value
	}
}

func applyMove(target map[string]interface{}, fromPath, toPath string) {
	fromParts := strings.Split(strings.Trim(fromPath, "/"), "/")
	toParts := strings.Split(strings.Trim(toPath, "/"), "/")
	var value interface{}
	if key, i, ok := isTwoPartArray(target, fromParts); ok {
		value = target[key].([]interface{})[i]
	} else {
		parent, key, isArr, i := traverseToParent(target, fromParts)
		if parent == nil {
			return
		}
		if isArr {
			value = getFromSlice(parent, key, i)
		} else {
			value = parent[key]
		}
	}
	applyRemove(target, fromParts)
	applyAdd(target, toParts, value)
}

func insertIntoSlice(parent map[string]interface{}, key string, index int, value interface{}) {
	if arr, ok := parent[key].([]interface{}); ok {
		if index < 0 || index > len(arr) {
			return
		}
		if index == len(arr) {
			parent[key] = append(arr, value)
		} else {
			parent[key] = append(arr[:index], append([]interface{}{value}, arr[index:]...)...)
		}
	} else {
		parent[key] = []interface{}{value}
	}
}

func removeFromSlice(parent map[string]interface{}, key string, index int) {
	if arr, ok := parent[key].([]interface{}); ok && index >= 0 && index < len(arr) {
		parent[key] = append(arr[:index], arr[index+1:]...)
	}
}

func replaceInSlice(parent map[string]interface{}, key string, index int, value interface{}) {
	if arr, ok := parent[key].([]interface{}); ok && index >= 0 && index < len(arr) {
		arr[index] = value
		parent[key] = arr
	}
}

func getFromSlice(parent map[string]interface{}, key string, index int) interface{} {
	if arr, ok := parent[key].([]interface{}); ok && index >= 0 && index < len(arr) {
		return arr[index]
	}
	return nil
}
