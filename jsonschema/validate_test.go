package jsonschema

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Phase 1: Core types and object/required/properties

func TestValidateAcceptsStringGivenTypeString(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString}
	err := Validate(schema, "hello")
	assert.NoError(t, err)
}

func TestValidateAcceptsNumberGivenTypeNumber(t *testing.T) {
	schema := map[string]any{TypeKey: TypeNumber}
	err := Validate(schema, 3.14)
	assert.NoError(t, err)
}

func TestValidateAcceptsIntegerGivenTypeInteger(t *testing.T) {
	schema := map[string]any{TypeKey: TypeInteger}
	err := Validate(schema, 42.0)
	assert.NoError(t, err)
}

func TestValidateAcceptsBooleanGivenTypeBoolean(t *testing.T) {
	schema := map[string]any{TypeKey: TypeBoolean}
	err := Validate(schema, true)
	assert.NoError(t, err)
}

func TestValidateAcceptsNullGivenTypeNull(t *testing.T) {
	schema := map[string]any{TypeKey: "null"}
	err := Validate(schema, nil)
	assert.NoError(t, err)
}

func TestValidateAcceptsObjectGivenTypeObject(t *testing.T) {
	schema := map[string]any{TypeKey: TypeObject}
	err := Validate(schema, map[string]any{})
	assert.NoError(t, err)
}

func TestValidateAcceptsArrayGivenTypeArray(t *testing.T) {
	schema := map[string]any{TypeKey: TypeArray}
	err := Validate(schema, []any{})
	assert.NoError(t, err)
}

func TestValidateRejectsWrongTypeWithPathAndMessage(t *testing.T) {
	schema := map[string]any{TypeKey: TypeNumber}
	err := Validate(schema, "not a number")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors(), 1)
	assert.Equal(t, "", verr.Errors()[0].Path)
	assert.Contains(t, verr.Errors()[0].Message, "number")
}

func TestValidateRequiredFailsWhenPropertyMissing(t *testing.T) {
	schema := map[string]any{
		TypeKey:     TypeObject,
		RequiredKey: []any{"a"},
		PropertiesKey: map[string]any{
			"a": map[string]any{TypeKey: TypeString},
		},
	}
	err := Validate(schema, map[string]any{})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors(), 1)
	assert.Contains(t, verr.Errors()[0].Message, "a")
}

func TestValidateRequiredPassesWhenPropertyPresent(t *testing.T) {
	schema := map[string]any{
		TypeKey:     TypeObject,
		RequiredKey: []any{"a"},
		PropertiesKey: map[string]any{
			"a": map[string]any{TypeKey: TypeString},
		},
	}
	err := Validate(schema, map[string]any{"a": "ok"})
	assert.NoError(t, err)
}

func TestValidatePropertiesPassesWhenValueMatchesSubschema(t *testing.T) {
	schema := map[string]any{
		TypeKey: TypeObject,
		PropertiesKey: map[string]any{
			"a": map[string]any{TypeKey: TypeString},
		},
	}
	err := Validate(schema, map[string]any{"a": "ok"})
	assert.NoError(t, err)
}

func TestValidatePropertiesFailsWhenValueViolatesSubschema(t *testing.T) {
	schema := map[string]any{
		TypeKey: TypeObject,
		PropertiesKey: map[string]any{
			"a": map[string]any{TypeKey: TypeString},
		},
	}
	err := Validate(schema, map[string]any{"a": 123})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors(), 1)
	assert.Equal(t, "/a", verr.Errors()[0].Path)
}

func TestValidateNestedObjectReportsPathCorrectly(t *testing.T) {
	schema := map[string]any{
		TypeKey: TypeObject,
		PropertiesKey: map[string]any{
			"outer": map[string]any{
				TypeKey: TypeObject,
				PropertiesKey: map[string]any{
					"inner": map[string]any{TypeKey: TypeInteger},
				},
			},
		},
	}
	data := map[string]any{
		"outer": map[string]any{"inner": "not an int"},
	}
	err := Validate(schema, data)
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors(), 1)
	assert.Equal(t, "/outer/inner", verr.Errors()[0].Path)
}

func TestValidateAcceptsNullableTypeGivenNull(t *testing.T) {
	schema := map[string]any{TypeKey: []any{TypeString, "null"}}
	err := Validate(schema, nil)
	assert.NoError(t, err)
}

func TestValidateAcceptsNullableTypeGivenNonNull(t *testing.T) {
	schema := map[string]any{TypeKey: []any{TypeString, "null"}}
	err := Validate(schema, "hello")
	assert.NoError(t, err)
}

// Phase 2: Array items and additionalProperties

func TestValidateItemsPassesWhenAllElementsMatch(t *testing.T) {
	schema := map[string]any{
		TypeKey:  TypeArray,
		ItemsKey: map[string]any{TypeKey: TypeString},
	}
	err := Validate(schema, []any{"a", "b"})
	assert.NoError(t, err)
}

func TestValidateItemsFailsWhenElementViolatesSubschema(t *testing.T) {
	schema := map[string]any{
		TypeKey:  TypeArray,
		ItemsKey: map[string]any{TypeKey: TypeString},
	}
	err := Validate(schema, []any{"a", 1})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors(), 1)
	assert.Equal(t, "/1", verr.Errors()[0].Path)
}

func TestValidateAdditionalPropertiesFalseFailsWhenExtraKey(t *testing.T) {
	schema := map[string]any{
		TypeKey:                 TypeObject,
		PropertiesKey:           map[string]any{"a": map[string]any{TypeKey: TypeString}},
		AdditionalPropertiesKey: false,
	}
	err := Validate(schema, map[string]any{"a": "ok", "b": "extra"})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors(), 1)
	assert.Contains(t, verr.Errors()[0].Message, "additional")
}

func TestValidateAdditionalPropertiesTrueAllowsExtraKeys(t *testing.T) {
	schema := map[string]any{
		TypeKey:                 TypeObject,
		PropertiesKey:           map[string]any{"a": map[string]any{TypeKey: TypeString}},
		AdditionalPropertiesKey: true,
	}
	err := Validate(schema, map[string]any{"a": "ok", "b": "extra"})
	assert.NoError(t, err)
}

// Phase 3: String/number constraints and enum/const

func TestValidateMinLengthFailsWhenTooShort(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString, MinLengthKey: 3.0}
	err := Validate(schema, "ab")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "minLength")
}

func TestValidateMaxLengthFailsWhenTooLong(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString, MaxLengthKey: 2.0}
	err := Validate(schema, "abc")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "maxLength")
}

func TestValidatePatternFailsWhenNoMatch(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString, PatternKey: `^[a-z]+$`}
	err := Validate(schema, "abc123")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "pattern")
}

func TestValidateMinimumFailsWhenLess(t *testing.T) {
	schema := map[string]any{TypeKey: TypeNumber, MinimumKey: 10.0}
	err := Validate(schema, 5.0)
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "minimum")
}

func TestValidateMaximumFailsWhenGreater(t *testing.T) {
	schema := map[string]any{TypeKey: TypeNumber, MaximumKey: 10.0}
	err := Validate(schema, 15.0)
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "maximum")
}

func TestValidateEnumFailsWhenNotInList(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString, EnumKey: []any{"a", "b", "c"}}
	err := Validate(schema, "d")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "enum")
}

func TestValidateEnumPassesWhenInList(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString, EnumKey: []any{"a", "b", "c"}}
	err := Validate(schema, "b")
	assert.NoError(t, err)
}

func TestValidateConstFailsWhenNotEqual(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString, ConstKey: "only"}
	err := Validate(schema, "other")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "const")
}

func TestValidateConstPassesWhenEqual(t *testing.T) {
	schema := map[string]any{TypeKey: TypeString, ConstKey: "only"}
	err := Validate(schema, "only")
	assert.NoError(t, err)
}

// Phase 4: Array constraints and $ref

func TestValidateMinItemsFailsWhenTooFew(t *testing.T) {
	schema := map[string]any{
		TypeKey:     TypeArray,
		ItemsKey:    map[string]any{TypeKey: TypeString},
		MinItemsKey: 2.0,
	}
	err := Validate(schema, []any{"a"})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "minItems")
}

func TestValidateMaxItemsFailsWhenTooMany(t *testing.T) {
	schema := map[string]any{
		TypeKey:     TypeArray,
		ItemsKey:    map[string]any{TypeKey: TypeString},
		MaxItemsKey: 2.0,
	}
	err := Validate(schema, []any{"a", "b", "c"})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "maxItems")
}

func TestValidateUniqueItemsFailsWhenDuplicate(t *testing.T) {
	schema := map[string]any{
		TypeKey:        TypeArray,
		ItemsKey:       map[string]any{TypeKey: TypeString},
		UniqueItemsKey: true,
	}
	err := Validate(schema, []any{"a", "a"})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Contains(t, verr.Errors()[0].Message, "uniqueItems")
}

func TestValidateRefResolvesAndValidates(t *testing.T) {
	schema := map[string]any{
		RefKey: "#/$defs/X",
		DefsKey: map[string]any{
			"X": map[string]any{TypeKey: TypeString},
		},
	}
	err := Validate(schema, "hello")
	assert.NoError(t, err)
	err = Validate(schema, 42)
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.Len(t, verr.Errors(), 1)
}

// Phase 5: allOf, anyOf, oneOf, not

func TestValidateAllOfPassesWhenAllMatch(t *testing.T) {
	schema := map[string]any{
		AllOfKey: []any{
			map[string]any{TypeKey: TypeObject},
			map[string]any{
				PropertiesKey: map[string]any{"a": map[string]any{TypeKey: TypeString}},
				RequiredKey:   []any{"a"},
			},
		},
	}
	err := Validate(schema, map[string]any{"a": "ok"})
	assert.NoError(t, err)
}

func TestValidateAllOfFailsWhenOneFails(t *testing.T) {
	schema := map[string]any{
		AllOfKey: []any{
			map[string]any{TypeKey: TypeObject},
			map[string]any{
				RequiredKey: []any{"a"},
				PropertiesKey: map[string]any{
					"a": map[string]any{TypeKey: TypeString},
				},
			},
		},
	}
	err := Validate(schema, map[string]any{})
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.GreaterOrEqual(t, len(verr.Errors()), 1)
}

func TestValidateAnyOfPassesWhenOneMatches(t *testing.T) {
	schema := map[string]any{
		AnyOfKey: []any{
			map[string]any{TypeKey: TypeInteger},
			map[string]any{TypeKey: TypeString},
		},
	}
	err := Validate(schema, "hello")
	assert.NoError(t, err)
}

func TestValidateAnyOfFailsWhenNoneMatch(t *testing.T) {
	schema := map[string]any{
		AnyOfKey: []any{
			map[string]any{TypeKey: TypeInteger},
			map[string]any{TypeKey: TypeBoolean},
		},
	}
	err := Validate(schema, "string")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
}

func TestValidateOneOfPassesWhenExactlyOneMatches(t *testing.T) {
	schema := map[string]any{
		OneOfKey: []any{
			map[string]any{TypeKey: TypeString},
			map[string]any{TypeKey: TypeInteger},
		},
	}
	err := Validate(schema, "hello")
	assert.NoError(t, err)
}

func TestValidateOneOfFailsWhenMultipleMatch(t *testing.T) {
	schema := map[string]any{
		OneOfKey: []any{
			map[string]any{TypeKey: TypeNumber},
			map[string]any{TypeKey: TypeInteger},
		},
	}
	err := Validate(schema, 42.0)
	require.Error(t, err)
}

func TestValidateNotPassesWhenSubschemaFails(t *testing.T) {
	schema := map[string]any{
		NotKey: map[string]any{TypeKey: "null"},
	}
	err := Validate(schema, "allowed")
	assert.NoError(t, err)
}

func TestValidateNotFailsWhenSubschemaPasses(t *testing.T) {
	schema := map[string]any{
		NotKey: map[string]any{TypeKey: TypeString},
	}
	err := Validate(schema, "disallowed")
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
}

// Roundtrip: generated schema validates decoded JSON

func TestValidateRoundtripGeneratedSchemaWithValidData(t *testing.T) {
	type T struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	schema := GenerateSchema(reflect.TypeOf(T{}))
	data := map[string]any{"id": 1.0, "name": "alice"}
	err := Validate(schema, data)
	assert.NoError(t, err)
}

func TestValidateRoundtripGeneratedSchemaWithInvalidData(t *testing.T) {
	type T struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	schema := GenerateSchema(reflect.TypeOf(T{}))
	data := map[string]any{"id": "not a number"}
	err := Validate(schema, data)
	require.Error(t, err)
	var verr *ErrValidation
	require.ErrorAs(t, err, &verr)
	assert.GreaterOrEqual(t, len(verr.Errors()), 1)
}
