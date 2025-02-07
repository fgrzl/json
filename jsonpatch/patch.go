package jsonpatch

import (
	"fmt"
	"reflect"
	"sort"
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

	// Process keys present in the "after" document.
	for key, afterVal := range afterMap {
		path := basePath + "/" + key
		beforeVal, exists := beforeMap[key]
		if !exists {
			// New key found: add operation.
			patches = append(patches, Patch{Op: "add", Path: path, Value: afterVal})
			continue
		}
		// If types differ, simply replace.
		if reflect.TypeOf(beforeVal) != reflect.TypeOf(afterVal) {
			patches = append(patches, Patch{Op: "replace", Path: path, Value: afterVal})
			continue
		}
		// Handle slices (arrays) specially.
		switch reflect.TypeOf(beforeVal).Kind() {
		case reflect.Slice:
			arrOps, err := generateArrayPatch(path, beforeVal, afterVal)
			if err != nil {
				return nil, err
			}
			patches = append(patches, arrOps...)
		// For maps or structs, recurse.
		case reflect.Map, reflect.Struct:
			nested, err := GeneratePatch(beforeVal, afterVal, path)
			if err != nil {
				return nil, err
			}
			patches = append(patches, nested...)
		default:
			// For simple types, compare with filtering.
			if !deepEqualFiltered(beforeVal, afterVal) {
				patches = append(patches, Patch{Op: "replace", Path: path, Value: afterVal})
			}
		}
	}

	// Process removals for keys that are in "before" but not in "after".
	for key := range beforeMap {
		if _, exists := afterMap[key]; !exists {
			patches = append(patches, Patch{Op: "remove", Path: basePath + "/" + key})
		}
	}
	return patches, nil
}

// ApplyPatch applies a series of JSON Patch operations to the original JSON object.
func ApplyPatch(original interface{}, patches []Patch) (interface{}, error) {
	target, err := toMap(original)
	if err != nil {
		return nil, err
	}
	// Process each patch sequentially.
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

// toMap converts a struct (or already a map) to a map[string]interface{} using reflection,
// thus avoiding expensive JSON round-trips.
func toMap(data interface{}) (map[string]interface{}, error) {
	if m, ok := data.(map[string]interface{}); ok {
		return m, nil
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	// Only structs are supported.
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported type %T for conversion to map", data)
	}
	result := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		// Use the json tag if available.
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

// deepEqualFiltered compares two values.
// For strings, it compares trimmed values to ignore incidental whitespace differences.
func deepEqualFiltered(a, b interface{}) bool {
	aStr, aOk := a.(string)
	bStr, bOk := b.(string)
	if aOk && bOk {
		return strings.TrimSpace(aStr) == strings.TrimSpace(bStr)
	}
	return reflect.DeepEqual(a, b)
}

// generateArrayPatch produces patch operations to transform one array into another.
// It first checks for a simple swap, then uses an improved diff based on the Longest Common
// Subsequence (LCS) to generate minimal operations.
func generateArrayPatch(basePath string, before, after interface{}) ([]Patch, error) {
	beforeSlice, err := toSlice(before)
	if err != nil {
		return nil, err
	}
	afterSlice, err := toSlice(after)
	if err != nil {
		return nil, err
	}

	// Check for a simple swap: if exactly two elements differ and are swapped.
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

	// Use the improved LCS-based diff algorithm.
	return improvedArrayDiff(basePath, beforeSlice, afterSlice)
}

// improvedArrayDiff computes the minimal set of "remove" and "add" operations to transform
// beforeSlice into afterSlice using LCS. When the arrays have equal lengths, any matching
// removal and addition at the same index are merged into a single "replace" operation.
// improvedArrayDiff generates a list of JSON Patch operations to transform
// beforeSlice into afterSlice. It uses the Longest Common Subsequence (LCS)
// algorithm to find the minimal set of changes.
//
// Parameters:
// - basePath: The base path for the JSON Patch operations.
// - beforeSlice: The original slice of interface{} elements.
// - afterSlice: The target slice of interface{} elements.
//
// Returns:
//   - A slice of Patch objects representing the necessary changes to transform
//     beforeSlice into afterSlice.
//   - An error if any issues occur during the process.
//
// The function performs the following steps:
//  1. Builds a DP table for the LCS of beforeSlice and afterSlice.
//  2. Determines which indices are part of the LCS.
//  3. Generates removal patches for elements in beforeSlice that are not in the LCS.
//  4. Generates addition patches for elements in afterSlice that are not in the LCS.
//  5. Combines removal and addition patches.
//  6. If beforeSlice and afterSlice are of equal length, merges matching remove+add
//     pairs into a "replace" operation.
func improvedArrayDiff(basePath string, beforeSlice, afterSlice []interface{}) ([]Patch, error) {
	m, n := len(beforeSlice), len(afterSlice)
	// Build DP table for LCS.
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if deepEqualFiltered(beforeSlice[i], afterSlice[j]) {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				if dp[i+1][j] >= dp[i][j+1] {
					dp[i][j] = dp[i+1][j]
				} else {
					dp[i][j] = dp[i][j+1]
				}
			}
		}
	}

	// Determine which indices are part of the LCS.
	commonBefore := make(map[int]bool)
	commonAfter := make(map[int]bool)
	i, j := 0, 0
	for i < m && j < n {
		if deepEqualFiltered(beforeSlice[i], afterSlice[j]) {
			commonBefore[i] = true
			commonAfter[j] = true
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}

	// Generate removal patches (in descending order).
	var removals []Patch
	for i := m - 1; i >= 0; i-- {
		if !commonBefore[i] {
			removals = append(removals, Patch{
				Op:   "remove",
				Path: basePath + "/" + strconv.Itoa(i),
			})
		}
	}

	// Generate addition patches (in ascending order).
	var additions []Patch
	for j := 0; j < n; j++ {
		if !commonAfter[j] {
			additions = append(additions, Patch{
				Op:    "add",
				Path:  basePath + "/" + strconv.Itoa(j),
				Value: afterSlice[j],
			})
		}
	}

	// Combine removals and additions.
	patches := append(removals, additions...)

	// If the arrays are equal in length, merge matching remove+add pairs into a "replace" op.
	if m == n {
		removalsMap := make(map[int]Patch)
		additionsMap := make(map[int]Patch)
		for _, p := range patches {
			idx, err := parseIndexFromPath(p.Path)
			if err != nil {
				continue
			}
			if p.Op == "remove" {
				removalsMap[idx] = p
			} else if p.Op == "add" {
				additionsMap[idx] = p
			}
		}
		mergedMap := make(map[int]Patch)
		for idx, remPatch := range removalsMap {
			if addPatch, ok := additionsMap[idx]; ok {
				// Merge into a replace operation.
				mergedMap[idx] = Patch{
					Op:    "replace",
					Path:  remPatch.Path,
					Value: addPatch.Value,
				}
			} else {
				mergedMap[idx] = remPatch
			}
		}
		for idx, addPatch := range additionsMap {
			if _, ok := mergedMap[idx]; !ok {
				mergedMap[idx] = addPatch
			}
		}
		// Build a sorted slice of merged patches.
		var merged []Patch
		for _, p := range mergedMap {
			merged = append(merged, p)
		}
		sort.Slice(merged, func(i, j int) bool {
			idx1, _ := parseIndexFromPath(merged[i].Path)
			idx2, _ := parseIndexFromPath(merged[j].Path)
			return idx1 < idx2
		})
		return merged, nil
	}

	return patches, nil
}

// parseIndexFromPath extracts the last segment of a JSON pointer path as an integer.
func parseIndexFromPath(path string) (int, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return -1, fmt.Errorf("no parts in path: %s", path)
	}
	return strconv.Atoi(parts[len(parts)-1])
}

// parsePath splits a JSON pointer path into its components.
func parsePath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	return strings.Split(strings.Trim(path, "/"), "/"), nil
}

// isTwoPartArray checks if the path has exactly two parts and that the first part
// refers to an array in the target object.
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

// traverseToParent walks the target object to the parent of the final key in the path.
func traverseToParent(target map[string]interface{}, parts []string) (map[string]interface{}, string, bool, int, error) {
	parent := target
	// Iterate through all but the last segment.
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		val, exists := parent[part]
		if !exists {
			return nil, "", false, -1, fmt.Errorf("path %s does not exist", strings.Join(parts[:i+1], "/"))
		}
		// If we're at the second-to-last segment and the value is an array,
		// then treat the next segment as an index.
		if i == len(parts)-2 {
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

// applyAdd applies an "add" operation at the given path with the specified value.
func applyAdd(target map[string]interface{}, parts []string, value interface{}) error {
	// Special handling for a two-part path that targets an array's end.
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
	// Otherwise, traverse to the parent container.
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

// insertIntoSlice inserts a value into a slice at the specified index.
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

// applyRemove applies a "remove" operation at the given path.
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

// removeFromSlice removes an element from a slice at the given index.
func removeFromSlice(parent map[string]interface{}, key string, index int) error {
	arr, ok := parent[key].([]interface{})
	if !ok || index < 0 || index >= len(arr) {
		return fmt.Errorf("invalid array or index %d", index)
	}
	parent[key] = append(arr[:index], arr[index+1:]...)
	return nil
}

// applyReplace applies a "replace" operation at the given path.
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

// replaceInSlice replaces an element in a slice at the specified index.
func replaceInSlice(parent map[string]interface{}, key string, index int, value interface{}) error {
	arr, ok := parent[key].([]interface{})
	if !ok || index < 0 || index >= len(arr) {
		return fmt.Errorf("invalid array or index %d", index)
	}
	arr[index] = value
	parent[key] = arr
	return nil
}

// applyMove applies a "move" operation from one path to another.
func applyMove(target map[string]interface{}, fromParts, toParts []string) error {
	var value interface{}
	// Retrieve the value from the "from" path.
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
	// Remove from the original location.
	if err := applyRemove(target, fromParts); err != nil {
		return err
	}
	// Add at the new location.
	return applyAdd(target, toParts, value)
}

// getFromSlice retrieves a value from a slice at the specified index.
func getFromSlice(parent map[string]interface{}, key string, index int) interface{} {
	arr, ok := parent[key].([]interface{})
	if !ok || index < 0 || index >= len(arr) {
		return nil
	}
	return arr[index]
}
