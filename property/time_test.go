package property

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestTime exercises the full Value contract for the Time type.
func TestTime(t *testing.T) {

	when := time.Date(2024, time.January, 22, 15, 4, 5, 0, time.UTC)
	value := Time(when)

	assert.True(t, value.IsTime())
	assert.True(t, when.Equal(value.Time()))
	assert.True(t, IsTime(value))

	// Scalar Value contract.
	assert.Equal(t, Nil{}, value.Get("anything"))
	assert.Equal(t, Map{"key": "v"}, value.Set("key", "v"))
	assert.Equal(t, value, value.Head())
	assert.Equal(t, Nil{}, value.Tail())
	assert.Equal(t, 1, value.Len())
	assert.Equal(t, map[string]any{}, value.Map())
	assert.Equal(t, value, value.Clone())

	// Raw round-trips to the underlying time.Time.
	assert.True(t, when.Equal(value.Raw().(time.Time)))

	// String emits an AS2-conformant date-time. This asserts the literal value
	// rather than round-tripping through datetime.Format, so a change to the
	// format shows up here instead of silently agreeing with itself.
	assert.Equal(t, "2024-01-22T15:04:05Z", value.String())
}

// TestTime_IsNil confirms Time reports nil only for the zero time.
func TestTime_IsNil(t *testing.T) {

	// A nil Time must serialize to an empty string, not a year-0001 timestamp.
	assert.True(t, Time(time.Time{}).IsNil())
	assert.Equal(t, "", Time(time.Time{}).String())
	assert.False(t, Time(time.Now()).IsNil())
}
