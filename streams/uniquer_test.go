package streams

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUniquer confirms IsUnique returns TRUE the first time it sees a value and
// FALSE on every subsequent sighting.
func TestUniquer(t *testing.T) {

	u := NewUniquer[string]()

	// First sighting is unique; all later sightings are not.
	assert.True(t, u.IsUnique("a"))
	assert.False(t, u.IsUnique("a"))
	assert.False(t, u.IsUnique("a"))

	// A different value is unique on its own first sighting.
	assert.True(t, u.IsUnique("b"))
	assert.False(t, u.IsUnique("b"))
}

// TestUniquer_IsDuplicate confirms IsDuplicate is the complement of a value's
// first sighting: false the first time, true thereafter. (IsDuplicate consumes
// the value, since it delegates to IsUnique.)
func TestUniquer_IsDuplicate(t *testing.T) {

	u := NewUniquer[string]()

	// First sighting: not a duplicate (but now recorded as seen).
	assert.False(t, u.IsDuplicate("a"))

	// Every sighting after that is a duplicate.
	assert.True(t, u.IsDuplicate("a"))
	assert.True(t, u.IsDuplicate("a"))
}

// TestUniquer_Int confirms the Uniquer works for any comparable type.
func TestUniquer_Int(t *testing.T) {

	u := NewUniquer[int]()
	assert.True(t, u.IsUnique(1))
	assert.True(t, u.IsUnique(2))
	assert.False(t, u.IsUnique(1))
}

// TestUniquer_Range confirms Range filters a sequence down to its first
// occurrences, preserving order.
func TestUniquer_Range(t *testing.T) {

	u := NewUniquer[string]()
	input := slices.Values([]string{"a", "b", "a", "c", "b", "a"})

	result := slices.Collect(u.Range(input))

	assert.Equal(t, []string{"a", "b", "c"}, result)
}

// TestUniquer_Range_EarlyStop confirms Range honors an early break from the
// consumer without seeing the rest of the sequence.
func TestUniquer_Range_EarlyStop(t *testing.T) {

	u := NewUniquer[int]()

	// Count how many values Range pulls from the sequence
	pulled := 0
	input := func(yield func(int) bool) {
		for _, value := range []int{1, 2, 3, 4, 5} {
			pulled++
			if !yield(value) {
				return
			}
		}
	}

	var collected []int
	for value := range u.Range(input) {
		collected = append(collected, value)
		if value == 2 {
			break
		}
	}

	assert.Equal(t, []int{1, 2}, collected)
	assert.Equal(t, 2, pulled)
}
