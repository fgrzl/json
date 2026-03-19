package jsonpatch

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Patch represents a single JSON Patch operation as defined by RFC 6902.
// The Op field is the operation (add, remove, replace, move). Path is
// the JSON Pointer location. From is used by move operations and Value
// holds the operation payload when applicable.
type Patch struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value"`
}

// GeneratePatch computes a list of JSON Patch operations that transform
// the `before` document into the `after` document. Both inputs may be
// structs or maps; basePath should be the JSON Pointer prefix (e.g.
// "" or "/root").
//
// The function attempts to produce minimal patches for arrays using an
// LCS-based algorithm. String comparison is exact (whitespace-sensitive).
func GeneratePatch(before, after any, basePath string) ([]Patch, error) {
	var patches []Patch
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
		beforeVal, exists := beforeMap[key]
		if !exists {
			patches = append(patches, Patch{Op: "add", Path: basePath + "/" + escapePathSegment(key), Value: afterVal})
			continue
		}
		// Nil edge case: reflect.TypeOf(nil) is nil and would panic on .Kind().
		if beforeVal == nil || afterVal == nil {
			if beforeVal != afterVal {
				patches = append(patches, Patch{Op: "replace", Path: basePath + "/" + escapePathSegment(key), Value: afterVal})
			}
			continue
		}
		// Fast path: skip path allocation for equal non-container types.
		switch beforeVal.(type) {
		case map[string]any, []any:
			// Containers need recursion; fall through to full handling below.
		default:
			if deepEqualFiltered(beforeVal, afterVal) {
				continue
			}
		}
		path := basePath + "/" + escapePathSegment(key)
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

	// Process removals for keys that are in "before" but not in "after".
	for key := range beforeMap {
		if _, exists := afterMap[key]; !exists {
			patches = append(patches, Patch{Op: "remove", Path: basePath + "/" + escapePathSegment(key)})
		}
	}
	return patches, nil
}

// ApplyPatch applies a series of JSON Patch operations to the original
// JSON-like object (struct or map). It returns the patched document as
// a map[string]any. The implementation applies operations sequentially
// and returns an error on the first failing operation.
func ApplyPatch(original any, patches []Patch) (map[string]any, error) {
	originalMap, err := toMap(original)
	if err != nil {
		return nil, err
	}

	// Create a deep copy to ensure atomicity
	target := deepCopy(originalMap)

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
		case "copy":
			fromParts, err := parsePath(op.From)
			if err != nil {
				return nil, err
			}
			err = applyCopy(target, fromParts, parts)
			if err != nil {
				return nil, err
			}
		case "test":
			err = applyTest(target, parts, op.Value)
		default:
			return nil, fmt.Errorf("unsupported op: %s", op.Op)
		}
		if err != nil {
			return nil, err
		}
	}
	return target, nil
}

// ApplyPatchAndHydrate applies patches to `original` and unmarshals the
// resulting document into `updated` (which should be a pointer). This is
// a convenience for applying patches and then hydrating a typed value.
func ApplyPatchAndHydrate(original, updated any, patches []Patch) error {
	// Convert original to map
	patched, err := ApplyPatch(original, patches)
	if err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}

	// Marshal patched map and unmarshal into `out`
	bytes, err := json.Marshal(patched)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := json.Unmarshal(bytes, updated); err != nil {
		return fmt.Errorf("unmarshal to %T: %w", updated, err)
	}
	return nil
}

// toMap converts a struct (or already a map) to a map[string]any using reflection,
// thus avoiding expensive JSON round-trips.
func toMap(data any) (map[string]any, error) {
	// If already a map
	if m, ok := data.(map[string]any); ok {
		return m, nil
	}
	// If a pointer to a map
	if m, ok := data.(*map[string]any); ok {
		return *m, nil
	}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return make(map[string]any), nil
		}
		v = v.Elem()
	}
	// Only structs are supported
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("unsupported type %T for conversion to map", data)
	}

	result := make(map[string]any, v.NumField())
	structToMap(v, result)
	return result, nil
}

// structToMap populates result with the fields of the struct value v,
// promoting anonymous (embedded) struct fields like encoding/json does.
func structToMap(v reflect.Value, result map[string]any) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported non-embedded fields
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}

		// Promote anonymous (embedded) struct fields
		if field.Anonymous {
			fv := v.Field(i)
			if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					continue
				}
				fv = fv.Elem()
			}
			if fv.Kind() == reflect.Struct {
				structToMap(fv, result)
				continue
			}
		}

		// Use JSON tag if available
		key := field.Name
		omitempty := false
		if tag := field.Tag.Get("json"); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				key = parts[0]
			}
			for _, p := range parts[1:] {
				if strings.TrimSpace(p) == "omitempty" {
					omitempty = true
					break
				}
			}
		}
		fv := v.Field(i)
		if omitempty && fv.IsZero() {
			continue
		}
		result[key] = convertValue(fv.Interface())
	}
}

// convertValue recursively converts structs to maps for consistent handling
func convertValue(data any) any {
	switch data.(type) {
	case nil:
		return nil
	case string, float64, int, int64, int32, bool:
		return data
	case map[string]any, []any:
		return data
	}

	if normalized, ok := normalizeSpecialValue(data); ok {
		return normalized
	}

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
		if normalized, ok := normalizeSpecialValue(v.Interface()); ok {
			return normalized
		}
	}

	switch v.Kind() {
	case reflect.Struct:
		// Convert nested struct to map
		m, err := toMap(data)
		if err != nil {
			return data // fallback to original value
		}
		return m
	case reflect.Slice, reflect.Array:
		// Convert slice/array elements
		result := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = convertValue(v.Index(i).Interface())
		}
		return result
	case reflect.Map:
		// Convert map values
		if v.Type().Key().Kind() == reflect.String {
			result := make(map[string]any)
			for _, key := range v.MapKeys() {
				keyStr := key.String()
				result[keyStr] = convertValue(v.MapIndex(key).Interface())
			}
			return result
		}
		return data
	default:
		return data
	}
}

func normalizeSpecialValue(data any) (any, bool) {
	if marshaler, ok := data.(json.Marshaler); ok {
		encoded, err := marshaler.MarshalJSON()
		if err == nil {
			var normalized any
			if err := json.Unmarshal(encoded, &normalized); err == nil {
				return normalized, true
			}
		}
	}

	if marshaler, ok := data.(encoding.TextMarshaler); ok {
		encoded, err := marshaler.MarshalText()
		if err == nil {
			return string(encoded), true
		}
	}

	return nil, false
}

// toSlice converts an array/slice to []any using reflection.
func toSlice(data any) ([]any, error) {
	if s, ok := data.([]any); ok {
		return s, nil
	}
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil, fmt.Errorf("unsupported type %T for conversion to slice", data)
	}
	result := make([]any, v.Len())
	for i := 0; i < v.Len(); i++ {
		result[i] = v.Index(i).Interface()
	}
	return result, nil
}

// sliceInsert inserts value at index into arr with a single allocation.
func sliceInsert(arr []any, index int, value any) []any {
	result := make([]any, len(arr)+1)
	copy(result, arr[:index])
	result[index] = value
	copy(result[index+1:], arr[index:])
	return result
}

// deepCopy creates a deep copy of a map[string]any structure
func deepCopy(original map[string]any) map[string]any {
	cp := make(map[string]any, len(original))
	for key, value := range original {
		switch v := value.(type) {
		case map[string]any:
			cp[key] = deepCopy(v)
		case []any:
			cp[key] = deepCopySlice(v)
		default:
			cp[key] = v
		}
	}
	return cp
}

// deepCopySlice creates a deep copy of a []any slice
func deepCopySlice(original []any) []any {
	copy := make([]any, len(original))
	for i, value := range original {
		switch v := value.(type) {
		case map[string]any:
			copy[i] = deepCopy(v)
		case []any:
			copy[i] = deepCopySlice(v)
		default:
			copy[i] = v
		}
	}
	return copy
}

// deepEqualFiltered compares two values.
// Fast-paths common JSON types to avoid reflect.DeepEqual overhead.
func deepEqualFiltered(a, b any) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	}
	return reflect.DeepEqual(a, b)
}

// generateArrayPatch produces patch operations to transform one array into another.
// It first checks for a simple swap, then uses an improved diff based on the Longest Common
// Subsequence (LCS) to generate minimal operations.
func generateArrayPatch(basePath string, before, after any) ([]Patch, error) {
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
	return arrayDiff(basePath, beforeSlice, afterSlice)
}

func arrayDiff(basePath string, beforeSlice, afterSlice []any) ([]Patch, error) {
	m, n := len(beforeSlice), len(afterSlice)

	// Precompute equality matrix so deepEqualFiltered is called at most m*n times.
	eq := make([]bool, m*n)
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			eq[i*n+j] = deepEqualFiltered(beforeSlice[i], afterSlice[j])
		}
	}

	// Build LCS DP using O(n) space (two rows).
	curr := make([]int, n+1)
	prev := make([]int, n+1)
	// We also need to reconstruct the LCS path, so store directional info.
	// direction: 0 = diagonal (match), 1 = up (skip before[i]), 2 = left (skip after[j])
	dir := make([]byte, (m+1)*(n+1))
	for i := m - 1; i >= 0; i-- {
		curr, prev = prev, curr
		for j := n - 1; j >= 0; j-- {
			if eq[i*n+j] {
				curr[j] = prev[j+1] + 1
				dir[i*(n+1)+j] = 0
			} else if prev[j] >= curr[j+1] {
				curr[j] = prev[j]
				dir[i*(n+1)+j] = 1
			} else {
				curr[j] = curr[j+1]
				dir[i*(n+1)+j] = 2
			}
		}
	}

	// Trace back the LCS using direction table.
	commonBefore := make([]bool, m)
	commonAfter := make([]bool, n)
	i, j := 0, 0
	for i < m && j < n {
		switch dir[i*(n+1)+j] {
		case 0:
			commonBefore[i] = true
			commonAfter[j] = true
			i++
			j++
		case 1:
			i++
		default:
			j++
		}
	}

	// Same-length arrays: positional replace is optimal (one patch per differing index).
	if m == n {
		var patches []Patch
		for i := 0; i < m; i++ {
			if !eq[i*n+i] {
				patches = append(patches, Patch{
					Op:    "replace",
					Path:  basePath + "/" + strconv.Itoa(i),
					Value: afterSlice[i],
				})
			}
		}
		return patches, nil
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

	return append(removals, additions...), nil
}

// parseIndexFromPath extracts the last segment of a JSON pointer path as an integer.
func parseIndexFromPath(path string) (int, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return -1, fmt.Errorf("no parts in path: %s", path)
	}
	return strconv.Atoi(parts[len(parts)-1])
}

// unescapePathSegment reverses JSON Pointer encoding per RFC 6901 section 3:
// ~1 -> /, ~0 -> ~ (order matters).
func unescapePathSegment(seg string) string {
	seg = strings.ReplaceAll(seg, "~1", "/")
	seg = strings.ReplaceAll(seg, "~0", "~")
	return seg
}

// escapePathSegment encodes a key for use in a JSON Pointer per RFC 6901:
// ~ -> ~0, / -> ~1.
func escapePathSegment(seg string) string {
	if !strings.ContainsAny(seg, "~/") {
		return seg
	}
	seg = strings.ReplaceAll(seg, "~", "~0")
	seg = strings.ReplaceAll(seg, "/", "~1")
	return seg
}

// parsePath splits a JSON pointer path into its components and unescapes each segment.
func parsePath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("empty path")
	}
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	// Fast path: most paths have no tilde escaping. Check once and skip per-segment work.
	needsUnescape := strings.Contains(trimmed, "~")
	for i, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("invalid path: contains empty component")
		}
		if needsUnescape {
			parts[i] = unescapePathSegment(part)
		}
	}
	return parts, nil
}

// isTwoPartArray checks if the path has exactly two parts and that the first part
// refers to an array in the target object.
func isTwoPartArray(target map[string]any, parts []string) (string, int, error) {
	if len(parts) != 2 {
		return "", -1, fmt.Errorf("expected 2 parts, got %d", len(parts))
	}
	key := parts[0]
	arr, ok := target[key].([]any)
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
// traverseToParent navigates to the parent container of the target path.
// For array indices, strictBounds controls whether out-of-bounds indices are allowed.
// When strictBounds is false, allows index == len(array) for append operations.
func traverseToParent(target map[string]any, parts []string) (map[string]any, string, bool, int, error) {
	return traverseToParentWithBounds(target, parts, true)
}

// traverseToParentForAdd is a variant for add operations that allows appending to arrays.
func traverseToParentForAdd(target map[string]any, parts []string) (map[string]any, string, bool, int, error) {
	return traverseToParentWithBounds(target, parts, false)
}

// traverseToParentWithBounds navigates to the parent container with configurable bounds checking.
func traverseToParentWithBounds(target map[string]any, parts []string, strictBounds bool) (map[string]any, string, bool, int, error) {
	parent := target
	// Iterate through all but the last segment.
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		val, exists := parent[part]
		if !exists {
			return nil, "", false, -1, fmt.Errorf("path %s does not exist", strings.Join(parts[:i+1], "/"))
		}

		// If we're at the second-to-last segment and the value is an array,
		// then treat the next segment as an index (or "-" for append when !strictBounds).
		if i == len(parts)-2 {
			if arr, ok := val.([]any); ok {
				var j int
				if parts[i+1] == "-" {
					if strictBounds {
						return nil, "", false, -1, fmt.Errorf("invalid index -")
					}
					j = len(arr)
				} else {
					var err error
					j, err = strconv.Atoi(parts[i+1])
					if err != nil {
						return nil, "", false, -1, fmt.Errorf("invalid index %s", parts[i+1])
					}
					// Check bounds based on strictBounds parameter
					if strictBounds {
						if j < 0 || j >= len(arr) {
							return nil, "", false, -1, fmt.Errorf("invalid index %s", parts[i+1])
						}
					} else {
						if j < 0 || j > len(arr) {
							return nil, "", false, -1, fmt.Errorf("invalid index %s", parts[i+1])
						}
					}
				}
				return parent, part, true, j, nil
			}
			// Otherwise, assume it's a map.
			if m, ok := val.(map[string]any); ok {
				parent = m
				return parent, parts[len(parts)-1], false, -1, nil
			}
			return nil, "", false, -1, fmt.Errorf("unexpected type at %s", part)
		} else {
			// Handle arrays in the middle of the path
			if arr, ok := val.([]any); ok {
				// Check if the next part is a valid array index
				if i+1 < len(parts) {
					if idx, err := strconv.Atoi(parts[i+1]); err == nil {
						if idx < 0 || idx >= len(arr) {
							return nil, "", false, -1, fmt.Errorf("index %d out of bounds", idx)
						}
						// Get the array element and continue traversal
						arrayElement := arr[idx]
						if m, ok := arrayElement.(map[string]any); ok {
							parent = m
							i++ // Skip the index part since we processed it
							continue
						} else {
							return nil, "", false, -1, fmt.Errorf("expected map at array index %d", idx)
						}
					} else {
						return nil, "", false, -1, fmt.Errorf("expected numeric index for array access, got %s", parts[i+1])
					}
				}
			} else if m, ok := val.(map[string]any); ok {
				parent = m
			} else {
				return nil, "", false, -1, fmt.Errorf("expected map at %s", part)
			}
		}
	}
	return parent, parts[len(parts)-1], false, -1, nil
} // applyAdd applies an "add" operation at the given path with the specified value.
func applyAdd(target map[string]any, parts []string, value any) error {
	// Special handling for a two-part path that targets an array (including /key/- for append).
	if len(parts) == 2 {
		if arr, ok := target[parts[0]].([]any); ok {
			var idx int
			if parts[1] == "-" {
				idx = len(arr)
			} else {
				n, err := strconv.Atoi(parts[1])
				if err != nil || n < 0 || n > len(arr) {
					if err != nil {
						return fmt.Errorf("invalid index %s", parts[1])
					}
					return fmt.Errorf("index %d out of bounds", n)
				}
				idx = n
			}
			if idx == len(arr) {
				target[parts[0]] = append(arr, value)
			} else {
				target[parts[0]] = sliceInsert(arr, idx, value)
			}
			return nil
		}
	}
	// Try direct two-part array handling (numeric index only; dash handled above).
	if key, idx, err := isTwoPartArray(target, parts); err == nil {
		arr := target[key].([]any)
		if idx == len(arr) {
			target[key] = append(arr, value)
		} else {
			target[key] = sliceInsert(arr, idx, value)
		}
		return nil
	}
	// Otherwise, traverse to the parent container with lenient bounds for add operations.
	parent, key, isArr, idx, err := traverseToParentForAdd(target, parts)
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
func insertIntoSlice(parent map[string]any, key string, index int, value any) error {
	arr, ok := parent[key].([]any)
	if !ok {
		parent[key] = []any{value}
		return nil
	}
	if index < 0 || index > len(arr) {
		return fmt.Errorf("index %d out of bounds", index)
	}
	if index == len(arr) {
		parent[key] = append(arr, value)
	} else {
		parent[key] = sliceInsert(arr, index, value)
	}
	return nil
}

// applyRemove applies a "remove" operation at the given path.
// RFC 6902 compliance: Must fail if the path does not exist.
func applyRemove(target map[string]any, parts []string) error {
	if key, idx, err := isTwoPartArray(target, parts); err == nil {
		arr, ok := target[key].([]any)
		if !ok {
			return fmt.Errorf("path %s does not exist or is not an array", strings.Join(parts, "/"))
		}
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

	// RFC 6902 compliance: Check if key exists before removing
	if _, exists := parent[key]; !exists {
		return fmt.Errorf("path %s does not exist", strings.Join(parts, "/"))
	}

	delete(parent, key)
	return nil
}

// removeFromSlice removes an element from a slice at the given index.
func removeFromSlice(parent map[string]any, key string, index int) error {
	arr, ok := parent[key].([]any)
	if !ok || index < 0 || index >= len(arr) {
		return fmt.Errorf("invalid array or index %d", index)
	}
	parent[key] = append(arr[:index], arr[index+1:]...)
	return nil
}

// applyReplace applies a "replace" operation at the given path.
// RFC 6902 compliance: Must fail if the path does not exist.
func applyReplace(target map[string]any, parts []string, value any) error {
	if key, idx, err := isTwoPartArray(target, parts); err == nil {
		arr, ok := target[key].([]any)
		if !ok {
			return fmt.Errorf("path %s does not exist or is not an array", strings.Join(parts, "/"))
		}
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

	// RFC 6902 compliance: Check if key exists before replacing
	if _, exists := parent[key]; !exists {
		return fmt.Errorf("path %s does not exist", strings.Join(parts, "/"))
	}

	parent[key] = value
	return nil
}

// replaceInSlice replaces an element in a slice at the specified index.
func replaceInSlice(parent map[string]any, key string, index int, value any) error {
	arr, ok := parent[key].([]any)
	if !ok || index < 0 || index >= len(arr) {
		return fmt.Errorf("invalid array or index %d", index)
	}
	arr[index] = value
	parent[key] = arr
	return nil
}

// isProperPrefix returns true if a is a proper prefix of b (same elements in order, shorter length).
func isProperPrefix(a, b []string) bool {
	if len(a) >= len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// applyMove applies a "move" operation from one path to another.
// RFC 6902 compliance: Must fail if the "from" path does not exist.
// RFC 6902 §4.4: from MUST NOT be a proper prefix of path.
func applyMove(target map[string]any, fromParts, toParts []string) error {
	if isProperPrefix(fromParts, toParts) {
		return fmt.Errorf("move failed: from path is a proper prefix of target path")
	}
	var value any
	// Retrieve the value from the "from" path.
	if key, idx, err := isTwoPartArray(target, fromParts); err == nil {
		arr, ok := target[key].([]any)
		if !ok {
			return fmt.Errorf("path %s does not exist or is not an array", strings.Join(fromParts, "/"))
		}
		if idx < 0 || idx >= len(arr) {
			return fmt.Errorf("index %d out of bounds", idx)
		}
		value = arr[idx]
	} else {
		parent, key, isArr, idx, err := traverseToParent(target, fromParts)
		if err != nil {
			return err
		}
		if isArr {
			var found bool
			value, found = getFromSlice(parent, key, idx)
			if !found {
				return fmt.Errorf("path %s does not exist", strings.Join(fromParts, "/"))
			}
		} else {
			// RFC 6902 compliance: Check if key exists before moving
			var exists bool
			if value, exists = parent[key]; !exists {
				return fmt.Errorf("path %s does not exist", strings.Join(fromParts, "/"))
			}
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
// It returns (value, true) when the index is in bounds (value may be nil);
// (nil, false) when the path does not exist or index is out of bounds.
func getFromSlice(parent map[string]any, key string, index int) (any, bool) {
	arr, ok := parent[key].([]any)
	if !ok || index < 0 || index >= len(arr) {
		return nil, false
	}
	return arr[index], true
}

// getValue retrieves the value at the given path parts from the target document.
// Returns (value, true) if found, (nil, false) otherwise.
func getValue(target map[string]any, parts []string) (any, bool) {
	if key, idx, err := isTwoPartArray(target, parts); err == nil {
		arr := target[key].([]any)
		return arr[idx], true
	}
	parent, key, isArr, idx, err := traverseToParent(target, parts)
	if err != nil {
		return nil, false
	}
	if isArr {
		return getFromSlice(parent, key, idx)
	}
	val, exists := parent[key]
	return val, exists
}

// applyTest checks that the value at the target path equals the expected value.
// RFC 6902 §4.6: the test fails if the value doesn't match.
func applyTest(target map[string]any, parts []string, expected any) error {
	actual, exists := getValue(target, parts)
	if !exists {
		return fmt.Errorf("test failed: path %s does not exist", strings.Join(parts, "/"))
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("test failed: value at %s is %v, expected %v", strings.Join(parts, "/"), actual, expected)
	}
	return nil
}

// applyCopy copies the value from one path to another without removing the source.
// RFC 6902 §4.5: equivalent to a get on from, then add at path.
func applyCopy(target map[string]any, fromParts, toParts []string) error {
	value, exists := getValue(target, fromParts)
	if !exists {
		return fmt.Errorf("path %s does not exist", strings.Join(fromParts, "/"))
	}
	// Deep-copy the value to prevent aliasing
	value = deepCopyValue(value)
	return applyAdd(target, toParts, value)
}

// deepCopyValue creates a deep copy of an arbitrary JSON-like value.
func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return deepCopy(val)
	case []any:
		return deepCopySlice(val)
	default:
		return v
	}
}
