package property

import (
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
)

// TestSlice exercises the Value contract for a populated Slice.
func TestSlice(t *testing.T) {

	value := Slice{"first", "second", "third"}

	assert.True(t, value.IsSlice())
	assert.Equal(t, []any{"first", "second", "third"}, value.Slice())
	assert.True(t, IsSlice(value))

	// Head is the first element (wrapped); Tail is everything after it.
	assert.Equal(t, String("first"), value.Head())
	assert.Equal(t, Slice{"second", "third"}, value.Tail())

	assert.Equal(t, 3, value.Len())
	assert.False(t, value.IsNil())
	assert.Equal(t, []any{"first", "second", "third"}, value.Raw())

	// String is intentionally empty for slices.
	assert.Equal(t, "", value.String())
}

// TestSlice_Map confirms Map delegates to the head element's Map representation.
func TestSlice_Map(t *testing.T) {

	value := Slice{
		map[string]any{vocab.PropertyName: "Alice"},
		map[string]any{vocab.PropertyName: "Bob"},
	}

	assert.Equal(t, map[string]any{vocab.PropertyName: "Alice"}, value.Map())
}

// TestSlice_Empty confirms an empty slice degrades gracefully to Nil for
// Head/Tail and reports itself as nil.
func TestSlice_Empty(t *testing.T) {

	value := Slice{}

	assert.Equal(t, Nil{}, value.Head())
	assert.Equal(t, Nil{}, value.Tail())
	assert.Equal(t, 0, value.Len())
	assert.True(t, value.IsNil())
}

// TestSlice_Get confirms Get reads a property from the first element of the slice.
func TestSlice_Get(t *testing.T) {

	value := Slice{
		map[string]any{vocab.PropertyName: "Alice"},
		map[string]any{vocab.PropertyName: "Bob"},
	}

	// Get delegates to the head element.
	assert.Equal(t, "Alice", value.Get(vocab.PropertyName).Raw())
}

// TestSlice_Set confirms Set writes a property onto the first element, leaving
// the tail untouched.
func TestSlice_Set(t *testing.T) {

	value := Slice{
		map[string]any{vocab.PropertyName: "Alice"},
		map[string]any{vocab.PropertyName: "Bob"},
	}

	result := value.Set(vocab.PropertyName, "Changed")

	assert.Equal(t, "Changed", result.Get(vocab.PropertyName).Raw())
	assert.Equal(t, 2, result.Len())
	assert.Equal(t, "Bob", result.Tail().Get(vocab.PropertyName).Raw())
}

// TestSlice_SetEmpty confirms setting a property on an empty slice produces a
// one-element slice carrying that property.
func TestSlice_SetEmpty(t *testing.T) {

	value := Slice{}
	result := value.Set(vocab.PropertyName, "Alice")

	assert.Equal(t, 1, result.Len())
	assert.Equal(t, "Alice", result.Get(vocab.PropertyName).Raw())
}

// TestSlice_Clone confirms a clone is independent of the original: mutating the
// clone's backing array must not affect the source.
func TestSlice_Clone(t *testing.T) {

	original := Slice{"a", "b", "c"}
	clone := original.Clone()

	// The clone starts out equal to the original.
	assert.Equal(t, original.Raw(), clone.Raw())

	// Mutating the clone's backing array must NOT affect the original.
	clone.Raw().([]any)[0] = "MUTATED"
	assert.Equal(t, "a", original[0], "Clone() must return an independent copy")
}
