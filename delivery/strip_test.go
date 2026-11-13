package delivery

import (
	"encoding/json"
	"testing"

	"github.com/benpate/hannibal/property"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/require"
)

// hasBlindProperty returns TRUE if `bto` or `bcc` appears anywhere in a decoded JSON value
func hasBlindProperty(value any) bool {

	switch typed := value.(type) {

	case map[string]any:
		for name, child := range typed {
			if isBlindProperty(name) || hasBlindProperty(child) {
				return true
			}
		}

	case []any:
		for _, child := range typed {
			if hasBlindProperty(child) {
				return true
			}
		}
	}

	return false
}

// roundTrip returns a value as encoding/json would decode it, so that every container has a plain type
func roundTrip(t *testing.T, value any) any {

	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	var decoded any
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	return decoded
}

// TestStripBlindRecipients_ThreeLevels verifies that the blind properties are removed from the
// activity, its object, and an object nested inside that
func TestStripBlindRecipients_ThreeLevels(t *testing.T) {

	activity := map[string]any{
		"type": "Create",
		"to":   []any{"https://example.com/@alice"},
		"bto":  []any{"https://example.com/@bob"},
		"bcc":  "https://example.com/@carol",
		"object": map[string]any{
			"type": "Note",
			"bcc":  []any{"https://example.com/@dave"},
			"cc":   []any{"https://example.com/@erin"},
			"inReplyTo": map[string]any{
				"id":  "https://example.com/notes/1",
				"bto": []any{"https://example.com/@frank"},
			},
		},
	}

	result := StripBlindRecipients(activity)

	require.False(t, hasBlindProperty(roundTrip(t, result)))
	require.Equal(t, []any{"https://example.com/@alice"}, result["to"])

	object, isMap := result["object"].(map[string]any)
	require.True(t, isMap)
	require.Equal(t, []any{"https://example.com/@erin"}, object["cc"])
	require.Equal(t, map[string]any{"id": "https://example.com/notes/1"}, object["inReplyTo"])
}

// TestStripBlindRecipients_LeavesTheOriginal verifies that the source document keeps its blind
// recipients, and shares no container with the copy
func TestStripBlindRecipients_LeavesTheOriginal(t *testing.T) {

	object := map[string]any{"bcc": []any{"https://example.com/@dave"}, "tag": []any{"one"}}
	activity := map[string]any{"bto": []any{"https://example.com/@bob"}, "object": object}

	result := StripBlindRecipients(activity)

	require.Contains(t, activity, "bto")
	require.Contains(t, object, "bcc")

	// Changing the copy must not reach the original
	copied, isMap := result["object"].(map[string]any)
	require.True(t, isMap)
	copied["added"] = true

	tags, isSlice := copied["tag"].([]any)
	require.True(t, isSlice)
	tags[0] = "changed"

	require.NotContains(t, object, "added")
	require.Equal(t, []any{"one"}, object["tag"])
}

// orderedDocument mimics a decoder's ordered document: a slice of key/value pairs
type orderedDocument []orderedPair

// orderedPair is one entry of an orderedDocument
type orderedPair struct {
	Key   string
	Value any
}

// driverArray mimics a database driver's array type, which no type switch on []any matches
type driverArray []any

// TestStripBlindRecipients_ContainerTypes verifies that no container type carries a blind
// property past the strip
func TestStripBlindRecipients_ContainerTypes(t *testing.T) {

	blind := []any{"https://example.com/@blind"}

	activity := map[string]any{
		"plainMaps":    []map[string]any{{"bcc": blind, "name": "a"}},
		"namedMap":     mapof.Any{"bto": blind, "name": "b"},
		"namedMaps":    []mapof.Any{{"bcc": blind, "name": "c"}},
		"propertyMap":  property.Map{"bto": blind, "name": "d"},
		"propertySet":  property.Slice{map[string]any{"bcc": blind, "name": "e"}},
		"namedSlice":   sliceof.Any{map[string]any{"bto": blind, "name": "f"}},
		"driverArray":  driverArray{map[string]any{"bcc": blind, "name": "g"}},
		"ordered":      orderedDocument{{Key: "bto", Value: blind}, {Key: "name", Value: "h"}},
		"orderedInner": orderedDocument{{Key: "inner", Value: map[string]any{"bcc": blind}}},
		"strings":      []string{"bto", "bcc"},
		"numberKeys":   map[int]string{1: "kept"},
	}

	result := StripBlindRecipients(activity)

	require.False(t, hasBlindProperty(roundTrip(t, result)))
	require.Equal(t, map[string]any{"name": "h"}, result["ordered"])
	require.Equal(t, []string{"bto", "bcc"}, result["strings"], "a value that merely equals the name is kept")
	require.Equal(t, map[int]string{1: "kept"}, result["numberKeys"])

	// Every name survives, so nothing but the blind properties was dropped
	encoded, err := json.Marshal(result)
	require.NoError(t, err)

	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		require.Contains(t, string(encoded), `"name":"`+name+`"`)
	}
}

// TestStripBlindRecipients_Empty verifies that a nil or empty document yields an empty copy
func TestStripBlindRecipients_Empty(t *testing.T) {
	require.Empty(t, StripBlindRecipients(nil))
	require.Empty(t, StripBlindRecipients(map[string]any{}))
	require.NotNil(t, StripBlindRecipients(nil))
}

// FuzzStripBlindRecipients verifies that no JSON document keeps a blind property at any depth,
// and that the strip never changes its input
func FuzzStripBlindRecipients(f *testing.F) {

	f.Add(`{"bto":["a"],"object":{"bcc":"b","inReplyTo":{"bto":["c"]}}}`)
	f.Add(`{"items":[{"bcc":[]},[{"bto":1}]],"bto":null}`)
	f.Add(`{}`)

	f.Fuzz(func(t *testing.T, input string) {

		var document map[string]any

		if err := json.Unmarshal([]byte(input), &document); err != nil {
			t.Skip()
		}

		before, err := json.Marshal(document)
		require.NoError(t, err)

		result := StripBlindRecipients(document)
		require.False(t, hasBlindProperty(roundTrip(t, result)))

		after, err := json.Marshal(document)
		require.NoError(t, err)
		require.Equal(t, string(before), string(after))
	})
}
