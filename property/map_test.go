package property

import (
	"sort"
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
)

// TestMap exercises the Value contract for a populated Map.
func TestMap(t *testing.T) {

	value := Map{
		vocab.PropertyID:   "https://example.com/1",
		vocab.PropertyName: "Alice",
	}

	assert.True(t, value.IsMap())
	assert.True(t, IsMap(value))
	assert.Equal(t, 1, value.Len()) // a map counts as a single element
	assert.False(t, value.IsNil())

	// Head is the map itself; Tail is Nil.
	assert.Equal(t, value, value.Head())
	assert.Equal(t, Nil{}, value.Tail())

	// Raw round-trips to the underlying map[string]any.
	assert.Equal(t, map[string]any(value), value.Raw())
	assert.Equal(t, map[string]any(value), value.Map())
}

// TestMap_Get confirms Get returns the wrapped value for present keys and Nil for absent.
func TestMap_Get(t *testing.T) {

	value := Map{vocab.PropertyName: "Alice"}

	assert.Equal(t, "Alice", value.Get(vocab.PropertyName).Raw())
	assert.Equal(t, Nil{}, value.Get(vocab.PropertyID))
}

// TestMap_Set confirms Set mutates the map in place and returns it.
func TestMap_Set(t *testing.T) {

	value := Map{}
	result := value.Set(vocab.PropertyName, "Alice")

	assert.Equal(t, "Alice", result.Get(vocab.PropertyName).Raw())
	// Set mutates in place, so the original sees the change too.
	assert.Equal(t, "Alice", value.Get(vocab.PropertyName).Raw())
}

// TestMap_String confirms String returns the id property rendered as a string.
func TestMap_String(t *testing.T) {

	value := Map{vocab.PropertyID: "https://example.com/1"}
	assert.Equal(t, "https://example.com/1", value.String())

	// A map with no id renders to an empty string.
	assert.Equal(t, "", Map{}.String())
}

// TestMap_IsNil confirms an empty map is nil and a populated one is not.
func TestMap_IsNil(t *testing.T) {
	assert.True(t, Map{}.IsNil())
	assert.False(t, Map{"x": 1}.IsNil())
}

// TestMap_MapKeys confirms MapKeys returns all keys (order-independent).
func TestMap_MapKeys(t *testing.T) {

	value := Map{"b": 2, "a": 1, "c": 3}
	keys := value.MapKeys()

	sort.Strings(keys)
	assert.Equal(t, []string{"a", "b", "c"}, keys)
}

// TestMap_Clone confirms a clone is independent of the original: mutating the
// clone must not affect the source.
func TestMap_Clone(t *testing.T) {

	original := Map{"key": "original"}
	clone := original.Clone()

	assert.Equal(t, original.Raw(), clone.Raw())

	// Mutating the clone must NOT affect the original.
	clone.Set("key", "MUTATED")
	assert.Equal(t, "original", original["key"], "Clone() must return an independent copy")
}
