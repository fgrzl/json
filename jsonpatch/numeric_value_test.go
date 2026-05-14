package jsonpatch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldNormalizeCommonNumericValuesGivenAnyNumericType(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{name: "float64", input: float64(1.5), want: 1.5, wantOK: true},
		{name: "float32", input: float32(2.5), want: 2.5, wantOK: true},
		{name: "int", input: int(-3), want: -3, wantOK: true},
		{name: "int8", input: int8(-4), want: -4, wantOK: true},
		{name: "int16", input: int16(-5), want: -5, wantOK: true},
		{name: "int32", input: int32(-6), want: -6, wantOK: true},
		{name: "int64", input: int64(-7), want: -7, wantOK: true},
		{name: "uint", input: uint(8), want: 8, wantOK: true},
		{name: "uint8", input: uint8(9), want: 9, wantOK: true},
		{name: "uint16", input: uint16(10), want: 10, wantOK: true},
		{name: "uint32", input: uint32(11), want: 11, wantOK: true},
		{name: "uint64", input: uint64(7), want: 7, wantOK: true},
		{name: "json number", input: json.Number("4.25"), want: 4.25, wantOK: true},
		{name: "json number parse failure", input: json.Number("not-a-number"), wantOK: false},
		{name: "unsupported type", input: "nope", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange

			// Act
			got, ok := numericValue(tt.input)

			// Assert
			if tt.wantOK {
				require.True(t, ok)
				assert.Equal(t, tt.want, got)
				return
			}

			assert.False(t, ok)
			assert.Zero(t, got)
		})
	}
}
