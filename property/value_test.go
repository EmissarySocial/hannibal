package property

import (
	"testing"
	"time"

	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewValue confirms NewValue wraps every supported raw type into the
// matching property.Value implementation.
func TestNewValue(t *testing.T) {

	check := func(name string, input any, expected Value) {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, NewValue(input))
		})
	}

	// nil collapses to the Nil sentinel.
	check("nil", nil, Nil{})

	// Raw scalar types.
	check("bool", true, Bool(true))
	check("float32", float32(1.5), Float(1.5))
	check("float64", float64(2.5), Float(2.5))
	check("int", 7, Int(7))
	check("int64", int64(9), Int64(9))
	check("string", "hello", String("hello"))

	// Maps -- both the std map and rosetta's mapof.Any wrap to Map.
	check("map[string]any", map[string]any{"a": 1}, Map{"a": 1})
	check("mapof.Any", mapof.Any{"a": 1}, Map{"a": 1})

	// Slices -- both []any and rosetta's sliceof.Any wrap to Slice.
	check("[]any", []any{1, 2}, Slice{1, 2})
	check("sliceof.Any", sliceof.Any{1, 2}, Slice{1, 2})
}

// TestNewValue_Time confirms NewValue wraps a time.Time as a Time holding the
// same instant.
func TestNewValue_Time(t *testing.T) {

	now := time.Now()
	result := NewValue(now)

	require.IsType(t, Time{}, result)
	assert.True(t, now.Equal(result.Raw().(time.Time)))
}

// TestNewValue_AlreadyValue confirms a Value passed back into NewValue is
// returned unchanged rather than re-wrapped.
func TestNewValue_AlreadyValue(t *testing.T) {

	original := String("already wrapped")
	assert.Equal(t, original, NewValue(original))
}

// TestNewValue_Unsupported confirms an unrecognized type collapses to Nil.
func TestNewValue_Unsupported(t *testing.T) {

	// A channel is neither a scalar, map, slice, nor a known getter interface.
	assert.Equal(t, Nil{}, NewValue(make(chan int)))
}

// The fixtures below each implement a single Getter interface, so that each
// getter branch of NewValue is exercised in isolation.

// boolGetterFixture implements only BoolGetter.
type boolGetterFixture struct{}

// Bool returns true.
func (boolGetterFixture) Bool() bool { return true }

// intGetterFixture implements only IntGetter.
type intGetterFixture struct{}

// Int returns 5.
func (intGetterFixture) Int() int { return 5 }

// int64GetterFixture implements only Int64Getter.
type int64GetterFixture struct{}

// Int64 returns 6.
func (int64GetterFixture) Int64() int64 { return 6 }

// floatGetterFixture implements only FloatGetter.
type floatGetterFixture struct{}

// Float returns 7.5.
func (floatGetterFixture) Float() float64 { return 7.5 }

// stringGetterFixture implements only StringGetter.
type stringGetterFixture struct{}

// String returns "from getter".
func (stringGetterFixture) String() string { return "from getter" }

// timeGetterFixture implements only TimeGetter.
type timeGetterFixture struct{}

// Time returns the Unix epoch in UTC.
func (timeGetterFixture) Time() time.Time { return time.Unix(0, 0).UTC() }

// mapGetterFixture implements only MapGetter.
type mapGetterFixture struct{}

// Map returns a one-key map, {"k": "v"}.
func (mapGetterFixture) Map() map[string]any { return map[string]any{"k": "v"} }

// sliceGetterFixture implements only SliceGetter.
type sliceGetterFixture struct{}

// Slice returns the two-element slice [1, 2].
func (sliceGetterFixture) Slice() []any { return []any{1, 2} }

// TestNewValue_Getters confirms NewValue honors the optional Getter interfaces,
// converting an arbitrary type into the matching property.Value.
func TestNewValue_Getters(t *testing.T) {

	assert.Equal(t, Bool(true), NewValue(boolGetterFixture{}))
	assert.Equal(t, Int(5), NewValue(intGetterFixture{}))
	assert.Equal(t, Int64(6), NewValue(int64GetterFixture{}))
	assert.Equal(t, Float(7.5), NewValue(floatGetterFixture{}))
	assert.Equal(t, String("from getter"), NewValue(stringGetterFixture{}))
	assert.Equal(t, Time(time.Unix(0, 0).UTC()), NewValue(timeGetterFixture{}))
	assert.Equal(t, Map{"k": "v"}, NewValue(mapGetterFixture{}))
	assert.Equal(t, Slice{1, 2}, NewValue(sliceGetterFixture{}))
}

// TestNewValue_ConvertFallback confirms the convert.IsMap / convert.IsSlice
// fallbacks catch map- and slice-shaped types that aren't the exact []any /
// map[string]any the explicit cases match (e.g. a typed map or slice).
func TestNewValue_ConvertFallback(t *testing.T) {

	// A map with a non-any value type isn't map[string]any, so it falls through
	// to the convert.IsMap fallback.
	typedMap := map[string]int{"a": 1}
	result := NewValue(typedMap)
	require.IsType(t, Map{}, result)
	assert.Equal(t, 1, result.Get("a").Raw())

	// Likewise a typed slice falls through to the convert.IsSlice fallback.
	typedSlice := []string{"x", "y"}
	sliceResult := NewValue(typedSlice)
	require.IsType(t, Slice{}, sliceResult)
	assert.Equal(t, Slice{"x", "y"}, sliceResult)
}

// TestIntrospection covers the package-level Is* type predicates for both the
// positive case (a value that reports the type) and the negative case.
func TestIntrospection(t *testing.T) {

	// Each predicate must report TRUE for its own type and FALSE for an
	// unrelated raw value (a plain string carries none of these interfaces).
	plain := "not introspectable"

	t.Run("IsBool", func(t *testing.T) {
		assert.True(t, IsBool(Bool(true)))
		assert.False(t, IsBool(plain))
	})

	t.Run("IsFloat", func(t *testing.T) {
		assert.True(t, IsFloat(Float(1.0)))
		assert.False(t, IsFloat(plain))
	})

	t.Run("IsInt", func(t *testing.T) {
		assert.True(t, IsInt(Int(1)))
		assert.False(t, IsInt(plain))
	})

	t.Run("IsInt64", func(t *testing.T) {
		assert.True(t, IsInt64(Int64(1)))
		assert.False(t, IsInt64(plain))
	})

	t.Run("IsMap", func(t *testing.T) {
		assert.True(t, IsMap(Map{}))
		assert.False(t, IsMap(plain))
	})

	t.Run("IsSlice", func(t *testing.T) {
		assert.True(t, IsSlice(Slice{}))
		assert.False(t, IsSlice(plain))
	})

	t.Run("IsString", func(t *testing.T) {
		assert.True(t, IsString(String("x")))
		assert.False(t, IsString(plain))
	})

	t.Run("IsTime", func(t *testing.T) {
		assert.True(t, IsTime(Time(time.Now())))
		assert.False(t, IsTime(plain))
	})
}

// FuzzNewValue ensures NewValue never panics on arbitrary string input and
// always returns a non-nil Value (the interface is never literally nil).
func FuzzNewValue(f *testing.F) {

	f.Add("")
	f.Add("hello")
	f.Add("123")

	f.Fuzz(func(t *testing.T, input string) {
		result := NewValue(input)
		require.NotNil(t, result)
		// A string always round-trips through String.
		assert.Equal(t, String(input), result)
	})
}
