package property

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewValue_EveryIntegerKind verifies that no integer kind is read as nil
func TestNewValue_EveryIntegerKind(t *testing.T) {

	for name, value := range map[string]any{
		"int8": int8(5), "int16": int16(5), "int32": int32(5), "uint8": uint8(5),
		"uint16": uint16(5), "uint32": uint32(5), "uint": uint(5), "uint64": uint64(5),
		"json.Number": json.Number("5"),
	} {
		wrapped := NewValue(value)
		require.False(t, wrapped.IsNil(), name)
		require.True(t, IsInt64(wrapped), name)
		require.Equal(t, int64(5), wrapped.(Int64Getter).Int64(), name)
	}

	// A document read back from a database holds an int32
	document := NewValue(map[string]any{"totalItems": int32(5)})
	require.Equal(t, int64(5), NewValue(document.Get("totalItems").Raw()).(Int64Getter).Int64())

	// The edges
	require.Equal(t, Int64(math.MaxUint32), NewValue(uint32(math.MaxUint32)))
	require.Equal(t, Int64(math.MaxInt64), NewValue(uint64(math.MaxInt64)))
	require.Equal(t, Float(float64(math.MaxUint64)), NewValue(uint64(math.MaxUint64)))
	require.Equal(t, Int(-128), NewValue(int8(-128)))
	require.Equal(t, Float(1.5), NewValue(json.Number("1.5")))
	require.Equal(t, Float(1e30), NewValue(json.Number("1e30")))
	require.Equal(t, Nil{}, NewValue(json.Number("not a number")))
	require.Equal(t, Nil{}, NewValue(json.Number("")))
}
