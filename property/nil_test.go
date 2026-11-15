package property

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNil confirms the Nil sentinel absorbs every operation: it gets/sets/heads/
// tails to itself and reports as empty.
func TestNil(t *testing.T) {

	value := Nil{}

	// Every navigation returns Nil again -- Nil is a fixed point.
	assert.Equal(t, Nil{}, value.Get("anything"))

	// Setting a property promotes Nil into a Map that holds it, as it does for a scalar
	assert.Equal(t, Map{"anything": "kept"}, value.Set("anything", "kept"))
	assert.Equal(t, Nil{}, value.Head())
	assert.Equal(t, Nil{}, value.Tail())
	assert.Equal(t, Nil{}, value.Clone())

	// Nil is empty: zero length, IsNil true, nil Raw, empty string and map.
	assert.Equal(t, 0, value.Len())
	assert.True(t, value.IsNil())
	assert.Equal(t, "", value.String())
	assert.Nil(t, value.Raw())
	assert.Equal(t, map[string]any{}, value.Map())
}
