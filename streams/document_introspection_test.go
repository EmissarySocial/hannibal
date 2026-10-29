package streams

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDocument_Introspection confirms which Is* predicates report TRUE for each kind of value,
// and that every other predicate reports FALSE.
func TestDocument_Introspection(t *testing.T) {

	predicates := map[string]func(Document) bool{
		"IsBool":   Document.IsBool,
		"IsInt":    Document.IsInt,
		"IsInt64":  Document.IsInt64,
		"IsFloat":  Document.IsFloat,
		"IsString": Document.IsString,
		"IsMap":    Document.IsMap,
		"IsSlice":  Document.IsSlice,
		"IsNil":    Document.IsNil,
	}

	// matches asserts that exactly the named predicates are true for this value.
	matches := func(name string, value any, expected ...string) {
		t.Run(name, func(t *testing.T) {
			document := NewDocument(value)
			for predicateName, predicate := range predicates {
				assert.Equal(t, slices.Contains(expected, predicateName), predicate(document), predicateName)
			}
		})
	}

	matches("bool", true, "IsBool")
	matches("int", 7, "IsInt", "IsInt64")
	matches("int64", int64(7), "IsInt", "IsInt64")
	matches("float", 3.14, "IsFloat")
	matches("string", "hello", "IsString")
	matches("map", map[string]any{"a": 1}, "IsMap")
	matches("slice", []any{1, 2}, "IsSlice")
	matches("nil", nil, "IsNil")

	// Zero values and empty containers also report IsNil, alongside their own type
	matches("false", false, "IsBool", "IsNil")
	matches("zero int", 0, "IsInt", "IsInt64", "IsNil")
	matches("zero float", 0.0, "IsFloat", "IsNil")
	matches("empty string", "", "IsString", "IsNil")
	matches("empty map", map[string]any{}, "IsMap", "IsNil")
	matches("empty slice", []any{}, "IsSlice", "IsNil")
}

// TestDocument_NilNotNil confirms IsNil/NotNil/NotEmpty/IsEmpty agree about
// emptiness for nil and populated documents.
func TestDocument_NilNotNil(t *testing.T) {

	nilDoc := NilDocument()
	assert.True(t, nilDoc.IsNil())
	assert.False(t, nilDoc.NotNil())
	assert.True(t, nilDoc.IsEmpty())
	assert.False(t, nilDoc.NotEmpty())

	populated := NewDocument(map[string]any{"id": "x"})
	assert.False(t, populated.IsNil())
	assert.True(t, populated.NotNil())
	assert.False(t, populated.IsEmpty())
	assert.True(t, populated.NotEmpty())
}

// TestDocument_Conversions confirms the scalar conversion accessors return the
// expected typed values.
func TestDocument_Conversions(t *testing.T) {

	t.Run("Bool", func(t *testing.T) {
		assert.True(t, NewDocument(true).Bool())
		assert.False(t, NewDocument(false).Bool())
	})

	t.Run("Int", func(t *testing.T) {
		assert.Equal(t, 42, NewDocument(42).Int())
	})

	t.Run("Float", func(t *testing.T) {
		assert.Equal(t, 3.5, NewDocument(3.5).Float())
	})

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "hello", NewDocument("hello").String())
	})

	t.Run("Time", func(t *testing.T) {
		when := time.Date(2024, time.January, 22, 0, 0, 0, 0, time.UTC)
		// Times round-trip through the W3C/HTTP string format.
		doc := NewDocument(when.Format("Mon, 02 Jan 2006 15:04:05 GMT"))
		assert.True(t, when.Equal(doc.Time()))
	})
}

// TestDocument_Value confirms Value returns the raw unwrapped payload.
func TestDocument_Value(t *testing.T) {
	assert.Equal(t, "hello", NewDocument("hello").Value())
	assert.Equal(t, 42, NewDocument(42).Value())
	assert.Nil(t, NilDocument().Value())
}

// TestDocument_String_Sanitizes confirms String() strips HTML (bluemonday
// strict policy) while HTMLString() keeps user-generated-content-safe markup.
func TestDocument_String_Sanitizes(t *testing.T) {

	doc := NewDocument("<b>bold</b> <script>evil()</script>")

	// Strict policy removes all tags; the inner text survives.
	assert.Equal(t, "bold ", doc.String())

	// UGC policy keeps safe formatting tags but still drops scripts.
	html := doc.HTMLString()
	assert.Contains(t, html, "<b>bold</b>")
	assert.NotContains(t, html, "<script>")
}
