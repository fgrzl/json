package jsonpatch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type helperMarshaledValue struct {
	Name string
}

func (v helperMarshaledValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"name": v.Name})
}

type helperTextValue string

func (v helperTextValue) MarshalText() ([]byte, error) {
	return []byte(string(v)), nil
}

type helperPromotedFields struct {
	Promoted  string `json:"promoted"`
	Ignored   string `json:"-"`
	OmitEmpty string `json:"omitEmpty,omitempty"`
}

type helperPointerFields struct {
	PointerPromoted string `json:"pointerPromoted"`
}

type helperNestedValue struct {
	Value string `json:"value"`
}

type helperConversionFixture struct {
	helperPromotedFields
	*helperPointerFields

	Name         string               `json:"name"`
	Count        int                  `json:"count"`
	Count64      int64                `json:"count64"`
	Count32      int32                `json:"count32"`
	Ratio        float64              `json:"ratio"`
	Enabled      bool                 `json:"enabled"`
	RawMap       map[string]any       `json:"rawMap"`
	RawSlice     []any                `json:"rawSlice"`
	TypedMap     map[string]int       `json:"typedMap"`
	NonStringMap map[int]string       `json:"nonStringMap"`
	Values       []int                `json:"values"`
	Pair         [2]string            `json:"pair"`
	Marshaled    helperMarshaledValue `json:"marshaled"`
	Text         helperTextValue      `json:"text"`
	PointerValue *helperNestedValue   `json:"pointerValue"`
	NilPointer   *helperNestedValue   `json:"nilPointer"`
	Unsupported  chan int             `json:"unsupported"`
	Omitted      string               `json:"omitted,omitempty"`
}

func TestShouldConvertStructValuesGivenMixedFieldKinds(t *testing.T) {
	tests := []struct {
		name                string
		anonymousPointer    *helperPointerFields
		wantPointerPromoted bool
	}{
		{name: "skips nil anonymous pointer", anonymousPointer: nil, wantPointerPromoted: false},
		{name: "promotes anonymous pointer", anonymousPointer: &helperPointerFields{PointerPromoted: "pointerPromoted"}, wantPointerPromoted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fixture := helperConversionFixture{
				helperPromotedFields: helperPromotedFields{Promoted: "promoted"},
				helperPointerFields:  tt.anonymousPointer,
				Name:                 "name",
				Count:                1,
				Count64:              64,
				Count32:              32,
				Ratio:                1.5,
				Enabled:              true,
				RawMap:               map[string]any{"raw": true},
				RawSlice:             []any{"raw", 2},
				TypedMap:             map[string]int{"typed": 3},
				NonStringMap:         map[int]string{1: "one"},
				Values:               []int{4, 5},
				Pair:                 [2]string{"left", "right"},
				Marshaled:            helperMarshaledValue{Name: "marshaled"},
				Text:                 helperTextValue("bravo"),
				PointerValue:         &helperNestedValue{Value: "pointer"},
				NilPointer:           nil,
				Unsupported:          make(chan int),
			}

			// Act
			gotAny, err := toMap(fixture)

			// Assert
			require.NoError(t, err)
			got := gotAny

			assert.Equal(t, "promoted", got["promoted"])
			assert.NotContains(t, got, "ignored")
			assert.NotContains(t, got, "omitEmpty")
			assert.NotContains(t, got, "omitted")
			assert.NotContains(t, got, "hidden")
			assert.Equal(t, "name", got["name"])
			assert.Equal(t, 1, got["count"])
			assert.Equal(t, int64(64), got["count64"])
			assert.Equal(t, int32(32), got["count32"])
			assert.Equal(t, 1.5, got["ratio"])
			assert.Equal(t, true, got["enabled"])
			assert.Equal(t, map[string]any{"raw": true}, got["rawMap"])
			assert.Equal(t, []any{"raw", 2}, got["rawSlice"])
			assert.Equal(t, map[string]any{"typed": 3}, got["typedMap"])
			assert.Equal(t, map[int]string{1: "one"}, got["nonStringMap"])
			assert.Equal(t, []any{4, 5}, got["values"])
			assert.Equal(t, []any{"left", "right"}, got["pair"])
			assert.Equal(t, map[string]any{"name": "marshaled"}, got["marshaled"])
			assert.Equal(t, "bravo", got["text"])
			assert.Equal(t, map[string]any{"value": "pointer"}, got["pointerValue"])
			assert.Nil(t, got["nilPointer"])
			assert.Equal(t, fixture.Unsupported, got["unsupported"])
			if tt.wantPointerPromoted {
				assert.Equal(t, "pointerPromoted", got["pointerPromoted"])
			} else {
				assert.NotContains(t, got, "pointerPromoted")
			}
		})
	}
}

func TestShouldConvertNilValuesGivenDirectHelpers(t *testing.T) {
	// Arrange
	var input any

	// Act
	got := convertValue(input)

	// Assert
	assert.Nil(t, got)
}

func TestShouldResolvePathHelpersGivenJSONPointerCases(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "parse path variants",
			fn: func(t *testing.T) {
				// Arrange
				// Act
				empty, err := parsePath("")
				escaped, escapedErr := parsePath("/a/~1b/~0c")
				_, invalidErr := parsePath("/a//b")

				// Assert
				require.NoError(t, err)
				assert.Empty(t, empty)
				require.NoError(t, escapedErr)
				assert.Equal(t, []string{"a", "/b", "~c"}, escaped)
				require.Error(t, invalidErr)
			},
		},
		{
			name: "two-part array helpers",
			fn: func(t *testing.T) {
				// Arrange
				target := map[string]any{"items": []any{"zero", "one"}}

				// Act
				key, idx, err := isTwoPartArray(target, []string{"items", "1"})
				_, _, wrongLenErr := isTwoPartArray(target, []string{"items"})
				_, _, nonArrayErr := isTwoPartArray(map[string]any{"items": "nope"}, []string{"items", "1"})
				_, _, badIndexErr := isTwoPartArray(target, []string{"items", "bad"})

				// Assert
				require.NoError(t, err)
				assert.Equal(t, "items", key)
				assert.Equal(t, 1, idx)
				require.Error(t, wrongLenErr)
				require.Error(t, nonArrayErr)
				require.Error(t, badIndexErr)
			},
		},
		{
			name: "slice lookups and getValue",
			fn: func(t *testing.T) {
				// Arrange
				target := map[string]any{
					"items": []any{"zero", map[string]any{"name": "one"}},
					"user":  map[string]any{"name": "alice"},
				}
				middleTarget := map[string]any{
					"parent": map[string]any{"items": []any{map[string]any{"name": "one"}}},
				}

				// Act
				value, ok := getFromSlice(target, "items", 1)
				_, missingSliceOK := getFromSlice(target, "missing", 0)
				_, nonSliceOK := getFromSlice(map[string]any{"items": "nope"}, "items", 0)
				root, rootOK := getValue(target, []string{})
				arrayValue, arrayOK := getValue(target, []string{"items", "1"})
				nestedValue, nestedOK := getValue(target, []string{"user", "name"})
				middleValue, middleOK := getValue(middleTarget, []string{"parent", "items", "0"})
				_, missingParentOK := getValue(map[string]any{"user": "nope"}, []string{"user", "name"})
				_, missingOK := getValue(target, []string{"user", "missing"})

				// Assert
				require.True(t, ok)
				assert.Equal(t, map[string]any{"name": "one"}, value)
				assert.False(t, missingSliceOK)
				assert.False(t, nonSliceOK)
				require.True(t, rootOK)
				assert.Equal(t, target, root)
				require.True(t, arrayOK)
				assert.Equal(t, map[string]any{"name": "one"}, arrayValue)
				require.True(t, nestedOK)
				assert.Equal(t, "alice", nestedValue)
				require.True(t, middleOK)
				assert.Equal(t, map[string]any{"name": "one"}, middleValue)
				assert.False(t, missingParentOK)
				assert.False(t, missingOK)
			},
		},
		{
			name: "traverse parent helpers",
			fn: func(t *testing.T) {
				// Arrange
				mapTarget := map[string]any{"user": map[string]any{"name": "alice"}}
				arrayTarget := map[string]any{"items": []any{map[string]any{"child": map[string]any{"leaf": "value"}}}}
				badArrayTarget := map[string]any{"items": []any{"leaf"}}
				strictDashTarget := map[string]any{"items": []any{"leaf"}}
				lenientBoundsTarget := map[string]any{"items": []any{"leaf"}}
				unexpectedTypeTarget := map[string]any{"items": "nope"}
				deepUnexpectedTypeTarget := map[string]any{"items": "nope"}

				// Act
				parent, key, isArr, idx, err := traverseToParent(mapTarget, []string{"user", "name"})
				_, _, _, _, missingErr := traverseToParent(map[string]any{}, []string{"missing", "name"})
				_, _, _, _, numericErr := traverseToParent(arrayTarget, []string{"items", "x", "child"})
				_, _, _, _, boundsErr := traverseToParent(map[string]any{"items": []any{"leaf"}}, []string{"items", "1", "child"})
				_, _, _, _, mapErr := traverseToParent(badArrayTarget, []string{"items", "0", "child"})
				_, _, _, _, unexpectedErr := traverseToParent(unexpectedTypeTarget, []string{"items", "child"})
				_, _, _, _, strictDashErr := traverseToParent(strictDashTarget, []string{"items", "-"})
				_, _, _, _, lenientBoundsErr := traverseToParentForAdd(lenientBoundsTarget, []string{"items", "2"})
				_, _, _, _, deepUnexpectedErr := traverseToParent(deepUnexpectedTypeTarget, []string{"items", "child", "leaf"})
				lenientParent, lenientKey, lenientIsArr, lenientIdx, lenientErr := traverseToParentForAdd(strictDashTarget, []string{"items", "-"})

				// Assert
				require.NoError(t, err)
				assert.Equal(t, mapTarget["user"], parent)
				assert.Equal(t, "name", key)
				assert.False(t, isArr)
				assert.Equal(t, -1, idx)
				require.Error(t, missingErr)
				require.Error(t, numericErr)
				require.Error(t, boundsErr)
				require.Error(t, mapErr)
				require.Error(t, unexpectedErr)
				require.Error(t, strictDashErr)
				require.Error(t, lenientBoundsErr)
				require.Error(t, deepUnexpectedErr)
				require.NoError(t, lenientErr)
				assert.Equal(t, strictDashTarget, lenientParent)
				assert.Equal(t, "items", lenientKey)
				assert.True(t, lenientIsArr)
				assert.Equal(t, 1, lenientIdx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestShouldManipulateSliceHelpersGivenInsertRemoveReplaceOperations(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "insert into slice variants",
			fn: func(t *testing.T) {
				// Arrange
				missing := map[string]any{}
				appendTarget := map[string]any{"items": []any{"a"}}
				middleTarget := map[string]any{"items": []any{"a", "c"}}
				nonSliceTarget := map[string]any{"items": "nope"}

				// Act
				require.NoError(t, insertIntoSlice(missing, "items", 0, "a"))
				require.NoError(t, insertIntoSlice(appendTarget, "items", 1, "b"))
				require.NoError(t, insertIntoSlice(middleTarget, "items", 1, "b"))
				require.NoError(t, insertIntoSlice(nonSliceTarget, "items", 0, "created"))
				invalidErr := insertIntoSlice(map[string]any{"items": []any{"a"}}, "items", 2, "b")

				// Assert
				assert.Equal(t, []any{"a"}, missing["items"])
				assert.Equal(t, []any{"a", "b"}, appendTarget["items"])
				assert.Equal(t, []any{"a", "b", "c"}, middleTarget["items"])
				assert.Equal(t, []any{"created"}, nonSliceTarget["items"])
				require.Error(t, invalidErr)
			},
		},
		{
			name: "remove from slice variants",
			fn: func(t *testing.T) {
				// Arrange
				parent := map[string]any{"items": []any{"a", "b"}}
				badParent := map[string]any{"items": "nope"}

				// Act
				require.NoError(t, removeFromSlice(parent, "items", 0))
				invalidIndexErr := removeFromSlice(map[string]any{"items": []any{"a"}}, "items", 1)
				nonSliceErr := removeFromSlice(badParent, "items", 0)

				// Assert
				assert.Equal(t, []any{"b"}, parent["items"])
				require.Error(t, invalidIndexErr)
				require.Error(t, nonSliceErr)
			},
		},
		{
			name: "replace in slice variants",
			fn: func(t *testing.T) {
				// Arrange
				parent := map[string]any{"items": []any{"a", "b"}}
				badParent := map[string]any{"items": "nope"}

				// Act
				require.NoError(t, replaceInSlice(parent, "items", 1, "c"))
				invalidIndexErr := replaceInSlice(map[string]any{"items": []any{"a"}}, "items", 1, "b")
				nonSliceErr := replaceInSlice(badParent, "items", 0, "b")

				// Assert
				assert.Equal(t, []any{"a", "c"}, parent["items"])
				require.Error(t, invalidIndexErr)
				require.Error(t, nonSliceErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestShouldApplyPatchOperationsGivenObjectAndArrayPaths(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "apply add variants",
			fn: func(t *testing.T) {
				// Arrange
				root := map[string]any{"old": "value"}
				arrayTarget := map[string]any{"items": []any{"a", "b"}}
				nestedTarget := map[string]any{"user": map[string]any{}}

				// Act
				require.NoError(t, applyAdd(root, []string{}, map[string]any{"fresh": "value"}))
				require.NoError(t, applyAdd(arrayTarget, []string{"items", "-"}, "c"))
				require.NoError(t, applyAdd(arrayTarget, []string{"items", "1"}, "x"))
				require.NoError(t, applyAdd(nestedTarget, []string{"user", "name"}, "alice"))

				// Assert
				assert.Equal(t, map[string]any{"fresh": "value"}, root)
				assert.Equal(t, []any{"a", "x", "b", "c"}, arrayTarget["items"])
				assert.Equal(t, map[string]any{"name": "alice"}, nestedTarget["user"])
			},
		},
		{
			name: "apply remove variants",
			fn: func(t *testing.T) {
				// Arrange
				arrayTarget := map[string]any{"items": []any{"a", "b"}}
				nestedTarget := map[string]any{"user": map[string]any{"name": "alice"}}

				// Act
				rootErr := applyRemove(map[string]any{"old": "value"}, []string{})
				require.NoError(t, applyRemove(arrayTarget, []string{"items", "0"}))
				require.NoError(t, applyRemove(nestedTarget, []string{"user", "name"}))
				missingErr := applyRemove(map[string]any{"user": map[string]any{}}, []string{"user", "name"})
				nonArrayErr := applyRemove(map[string]any{"items": "nope"}, []string{"items", "0"})
				oobErr := applyRemove(map[string]any{"items": []any{"a"}}, []string{"items", "1"})

				// Assert
				require.Error(t, rootErr)
				assert.Equal(t, []any{"b"}, arrayTarget["items"])
				assert.Equal(t, map[string]any{}, nestedTarget["user"])
				require.Error(t, missingErr)
				require.Error(t, nonArrayErr)
				require.Error(t, oobErr)
			},
		},
		{
			name: "apply replace variants",
			fn: func(t *testing.T) {
				// Arrange
				root := map[string]any{"old": "value"}
				arrayTarget := map[string]any{"items": []any{"a", "b"}}
				nestedTarget := map[string]any{"user": map[string]any{"name": "alice"}}

				// Act
				require.NoError(t, applyReplace(root, []string{}, map[string]any{"fresh": "value"}))
				require.NoError(t, applyReplace(arrayTarget, []string{"items", "1"}, "c"))
				require.NoError(t, applyReplace(nestedTarget, []string{"user", "name"}, "bob"))
				rootScalarErr := applyReplace(map[string]any{"old": "value"}, []string{}, "value")
				missingErr := applyReplace(map[string]any{"user": map[string]any{}}, []string{"user", "name"}, "bob")
				nonArrayErr := applyReplace(map[string]any{"items": "nope"}, []string{"items", "0"}, "c")
				oobErr := applyReplace(map[string]any{"items": []any{"a"}}, []string{"items", "1"}, "c")

				// Assert
				assert.Equal(t, map[string]any{"fresh": "value"}, root)
				assert.Equal(t, []any{"a", "c"}, arrayTarget["items"])
				assert.Equal(t, map[string]any{"name": "bob"}, nestedTarget["user"])
				require.Error(t, rootScalarErr)
				require.Error(t, missingErr)
				require.Error(t, nonArrayErr)
				require.Error(t, oobErr)
			},
		},
		{
			name: "apply move variants",
			fn: func(t *testing.T) {
				// Arrange
				moveTarget := map[string]any{"items": []any{"a", "b", "c"}, "user": map[string]any{"name": "alice"}}

				// Act
				rootErr := applyMove(map[string]any{"old": "value"}, []string{}, []string{"new"})
				prefixErr := applyMove(map[string]any{"user": map[string]any{"name": "alice"}}, []string{"user"}, []string{"user", "name"})
				missingErr := applyMove(map[string]any{"user": map[string]any{}}, []string{"user", "missing"}, []string{"copy"})
				nonArrayErr := applyMove(map[string]any{"items": "nope"}, []string{"items", "0"}, []string{"moved"})
				oobErr := applyMove(map[string]any{"items": []any{"a"}}, []string{"items", "1"}, []string{"moved"})
				require.NoError(t, applyMove(moveTarget, []string{"items", "0"}, []string{"moved"}))

				// Assert
				require.Error(t, rootErr)
				require.Error(t, prefixErr)
				require.Error(t, missingErr)
				require.Error(t, nonArrayErr)
				require.Error(t, oobErr)
				assert.Equal(t, map[string]any{"name": "alice"}, moveTarget["user"])
				assert.Equal(t, []any{"b", "c"}, moveTarget["items"])
				assert.Equal(t, "a", moveTarget["moved"])
			},
		},
		{
			name: "apply test and copy variants",
			fn: func(t *testing.T) {
				// Arrange
				target := map[string]any{
					"user": map[string]any{"name": "alice", "tags": []any{"go", "dev"}},
				}

				// Act
				require.NoError(t, applyTest(target, []string{}, struct {
					User map[string]any `json:"user"`
				}{User: map[string]any{"name": "alice", "tags": []any{"go", "dev"}}}))
				rootMismatchErr := applyTest(target, []string{}, struct {
					User map[string]any `json:"user"`
				}{User: map[string]any{"name": "bob"}})
				missingErr := applyTest(target, []string{"user", "missing"}, "value")
				pathMismatchErr := applyTest(target, []string{"user", "name"}, "bob")
				require.NoError(t, applyCopy(target, []string{"user"}, []string{"copiedUser"}))
				copyMissingErr := applyCopy(target, []string{"user", "missing"}, []string{"copiedMissing"})

				// Assert
				require.Error(t, rootMismatchErr)
				require.Error(t, missingErr)
				require.Error(t, pathMismatchErr)
				require.Error(t, copyMissingErr)
				assert.Equal(t, map[string]any{"name": "alice", "tags": []any{"go", "dev"}}, target["copiedUser"])
				copied := target["copiedUser"].(map[string]any)
				copied["name"] = "changed"
				assert.Equal(t, "alice", target["user"].(map[string]any)["name"])
			},
		},
		{
			name: "apply patch rejects invalid from paths",
			fn: func(t *testing.T) {
				// Arrange
				original := map[string]any{"src": "value"}

				// Act
				_, moveErr := ApplyPatch(original, []Patch{{Op: "move", Path: "/dest", From: "/bad//path"}})
				_, copyErr := ApplyPatch(original, []Patch{{Op: "copy", Path: "/dest", From: "/bad//path"}})

				// Assert
				require.Error(t, moveErr)
				require.Error(t, copyErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestShouldCopyAndCompareJSONValuesGivenNestedContainers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "deep copy values",
			fn: func(t *testing.T) {
				// Arrange
				mapSource := map[string]any{
					"map": map[string]any{
						"slice": []any{map[string]any{"key": "value"}, []any{1, 2}},
					},
				}
				sliceSource := []any{map[string]any{"key": "value"}, []any{1, 2}}

				// Act
				cloned := deepCopyValue(mapSource).(map[string]any)
				clonedSlice := deepCopyValue(sliceSource).([]any)
				scalar := deepCopyValue("value")

				// Assert
				mapSource["map"].(map[string]any)["slice"].([]any)[0].(map[string]any)["key"] = "changed"
				mapSource["map"].(map[string]any)["slice"].([]any)[1].([]any)[0] = 99
				assert.Equal(t, "value", cloned["map"].(map[string]any)["slice"].([]any)[0].(map[string]any)["key"])
				assert.Equal(t, []any{1, 2}, cloned["map"].(map[string]any)["slice"].([]any)[1])
				clonedSlice[0].(map[string]any)["key"] = "changed"
				assert.Equal(t, "value", sliceSource[0].(map[string]any)["key"])
				assert.Equal(t, "value", scalar)
			},
		},
		{
			name: "compare json values",
			fn: func(t *testing.T) {
				// Arrange
				tests := []struct {
					name string
					a    any
					b    any
					want bool
				}{
					{name: "nil", a: nil, b: nil, want: true},
					{name: "numeric equivalence", a: 1, b: 1.0, want: true},
					{name: "strings", a: "hello", b: "hello", want: true},
					{name: "booleans", a: true, b: true, want: true},
					{name: "bool mismatch", a: true, b: false, want: false},
					{name: "slices", a: []any{"a", float64(1), map[string]any{"b": "c"}}, b: []any{"a", float64(1), map[string]any{"b": "c"}}, want: true},
					{name: "slice length mismatch", a: []any{"a"}, b: []any{"a", "b"}, want: false},
					{name: "maps", a: map[string]any{"a": []any{float64(1), "x"}}, b: map[string]any{"a": []any{float64(1), "x"}}, want: true},
					{name: "struct fallback", a: struct{ ID int }{ID: 1}, b: struct{ ID int }{ID: 1}, want: true},
					{name: "mismatch", a: map[string]any{"a": true}, b: map[string]any{"a": false}, want: false},
				}

				// Act / Assert
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						assert.Equal(t, tt.want, jsonEqual(tt.a, tt.b))
					})
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}

func TestShouldGenerateArrayPatchesGivenCommonArrayShapes(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "trim common edges",
			fn: func(t *testing.T) {
				// Arrange
				before := []any{"a", "b", "c", "d"}
				after := []any{"a", "x", "c", "d"}

				// Act
				prefix, beforeMid, afterMid := trimCommonArrayEdges(before, after)

				// Assert
				assert.Equal(t, 1, prefix)
				assert.Equal(t, []any{"b"}, beforeMid)
				assert.Equal(t, []any{"x"}, afterMid)
			},
		},
		{
			name: "same-length middle uses replace patches",
			fn: func(t *testing.T) {
				// Arrange
				before := []any{"a", "b", "c"}
				after := []any{"a", "x", "c"}

				// Act
				patches, err := arrayDiff("/items", before, after)

				// Assert
				require.NoError(t, err)
				require.Len(t, patches, 1)
				assert.Equal(t, "replace", patches[0].Op)
				assert.Equal(t, "/items/1", patches[0].Path)
				assert.Equal(t, "x", patches[0].Value)
			},
		},
		{
			name: "unequal-length middles use add and remove patches",
			fn: func(t *testing.T) {
				// Arrange
				before := []any{"a", "b", "c"}
				after := []any{"a", "c"}

				// Act
				patches, err := arrayDiff("/items", before, after)

				// Assert
				require.NoError(t, err)
				require.NotEmpty(t, patches)
				assert.Equal(t, "remove", patches[0].Op)
				assert.Equal(t, "/items/1", patches[0].Path)
			},
		},
		{
			name: "generate array patch handles swap and invalid input",
			fn: func(t *testing.T) {
				// Arrange
				swapBefore := []any{1, 2, 3}
				swapAfter := []any{2, 1, 3}
				lcsBefore := []any{"a", "b", "c", "d"}
				lcsAfter := []any{"a", "x", "c", "d", "e"}

				// Act
				swapPatch, swapErr := generateArrayPatch("/items", swapBefore, swapAfter)
				lcsPatch, lcsErr := generateArrayPatch("/items", lcsBefore, lcsAfter)
				_, beforeErr := generateArrayPatch("/items", make(chan int), swapAfter)
				_, afterErr := generateArrayPatch("/items", swapBefore, make(chan int))

				// Assert
				require.NoError(t, swapErr)
				require.Len(t, swapPatch, 1)
				assert.Equal(t, "move", swapPatch[0].Op)
				require.NoError(t, lcsErr)
				require.NotEmpty(t, lcsPatch)
				require.Error(t, beforeErr)
				require.Error(t, afterErr)
			},
		},
		{
			name: "array diff handles no-op arrays",
			fn: func(t *testing.T) {
				// Arrange
				items := []any{"a", "b"}

				// Act
				patches, err := arrayDiff("/items", items, items)

				// Assert
				require.NoError(t, err)
				assert.Nil(t, patches)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}
