package hannibal

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/require"
)

// withoutBlindRecipients calls StripBlindRecipients, and fails the test if it returns an error
func withoutBlindRecipients(t *testing.T, activity map[string]any) map[string]any {

	t.Helper()

	result, err := StripBlindRecipients(activity)
	require.NoError(t, err)

	return result
}

// serialized returns a value as JSON text, which is what a recipient receives
func serialized(t *testing.T, value any) string {

	t.Helper()

	result, err := json.Marshal(value)
	require.NoError(t, err)

	return string(result)
}

// hasBlindProperty returns TRUE if `bto` or `bcc` appears as a property name anywhere in a
// value, read the way a recipient would read it
func hasBlindProperty(t *testing.T, value any) bool {

	t.Helper()

	var decoded any
	require.NoError(t, json.Unmarshal([]byte(serialized(t, value)), &decoded))

	return decodedHasBlindProperty(decoded)
}

// decodedHasBlindProperty searches a decoded JSON value for a `bto` or `bcc` property
func decodedHasBlindProperty(value any) bool {

	switch typed := value.(type) {

	case map[string]any:
		for name, child := range typed {
			if (name == vocab.PropertyBTo) || (name == vocab.PropertyBCC) || decodedHasBlindProperty(child) {
				return true
			}
		}

	case []any:
		for _, child := range typed {
			if decodedHasBlindProperty(child) {
				return true
			}
		}
	}

	return false
}

// TestBlindRecipients_ThreeLevels verifies that the blind properties are removed from the
// activity, its object, and an object nested inside that
func TestBlindRecipients_ThreeLevels(t *testing.T) {

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

	result := withoutBlindRecipients(t, activity)

	require.Equal(t, map[string]any{
		"type": "Create",
		"to":   []any{"https://example.com/@alice"},
		"object": map[string]any{
			"type":      "Note",
			"cc":        []any{"https://example.com/@erin"},
			"inReplyTo": map[string]any{"id": "https://example.com/notes/1"},
		},
	}, result)
}

// TestBlindRecipients_LeavesTheOriginal verifies that the source document keeps its blind
// recipients, and shares no container with the copy
func TestBlindRecipients_LeavesTheOriginal(t *testing.T) {

	object := map[string]any{"bcc": []any{"https://example.com/@dave"}, "tag": []any{"one"}}
	activity := map[string]any{"bto": []any{"https://example.com/@bob"}, "object": object}

	before := serialized(t, activity)
	result := withoutBlindRecipients(t, activity)

	// Changing the copy must not reach the original
	copied, isMap := result["object"].(map[string]any)
	require.True(t, isMap)
	copied["added"] = true

	tags, isSlice := copied["tag"].([]any)
	require.True(t, isSlice)
	tags[0] = "changed"

	require.Equal(t, before, serialized(t, activity))
	require.Contains(t, activity, "bto")
	require.Contains(t, object, "bcc")
}

// driverArray mimics a database driver's array type, which no type switch on []any matches
type driverArray []any

// driverMap mimics a database driver's document type
type driverMap map[string]any

// TestBlindRecipients_ContainerTypes verifies that no map or list type carries a blind
// property past the strip
func TestBlindRecipients_ContainerTypes(t *testing.T) {

	blind := []any{"https://example.com/@blind"}

	activity := map[string]any{
		"plainMaps":   []map[string]any{{"bcc": blind, "name": "a"}},
		"namedMap":    mapof.Any{"bto": blind, "name": "b"},
		"namedMaps":   []mapof.Any{{"bcc": blind, "name": "c"}},
		"propertyMap": property.Map{"bto": blind, "name": "d"},
		"propertySet": property.Slice{map[string]any{"bcc": blind, "name": "e"}},
		"namedSlice":  sliceof.Any{map[string]any{"bto": blind, "name": "f"}},
		"driverArray": driverArray{driverMap{"bcc": blind, "name": "g"}},
		"driverMap":   driverMap{"bto": blind, "name": "h", "items": driverArray{driverMap{"bcc": blind, "name": "i"}}},
		"array":       [1]any{map[string]any{"bto": blind, "name": "j"}},
		"mapOfLists":  map[string][]any{"bcc": blind, "kept": {map[string]any{"bto": blind, "name": "k"}}},
		"strings":     []string{"bto", "bcc"},
	}

	result := withoutBlindRecipients(t, activity)

	require.False(t, hasBlindProperty(t, result))
	require.Equal(t, []string{"bto", "bcc"}, result["strings"], "a value that merely equals the name is kept")

	// Every name survives, so nothing but the blind properties was dropped
	encoded := serialized(t, result)

	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"} {
		require.Contains(t, encoded, `"name":"`+name+`"`)
	}

	require.NotContains(t, encoded, "@blind")
}

// TestBlindRecipients_Empty verifies that a nil or empty document yields an empty copy
func TestBlindRecipients_Empty(t *testing.T) {

	require.Empty(t, withoutBlindRecipients(t, nil))
	require.Empty(t, withoutBlindRecipients(t, map[string]any{}))
	require.Equal(t, "{}", serialized(t, withoutBlindRecipients(t, nil)))
}

// TestBlindRecipients_KeepsTypes verifies that the copy holds every value as the type the
// original held, so that it can be stored as well as sent
func TestBlindRecipients_KeepsTypes(t *testing.T) {

	published := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	activity := map[string]any{
		"large":     int64(9007199254740993),
		"small":     3,
		"fraction":  1.5,
		"unsigned":  uint64(18446744073709551615),
		"published": published,
		"list":      []any{int32(7), map[string]any{"nested": uint8(8)}},
		"named":     mapof.Any{"tags": sliceof.String{"a"}},
		"driver":    driverMap{"items": driverArray{driverMap{"name": "g"}}},
	}

	result := withoutBlindRecipients(t, activity)

	require.Equal(t, activity, result)
	require.Contains(t, serialized(t, result), `"large":9007199254740993`)
	require.Contains(t, serialized(t, result), `"unsigned":18446744073709551615`)
}

// hiddenObject mimics an application's own struct, embedded in an activity
type hiddenObject struct {
	Name string   `json:"name"`
	BCC  []string `json:"bcc"`
}

// TestBlindRecipients_RefusesWhatItCannotSearch verifies that a value which could carry a blind
// recipient past the strip is an error, and that nothing is returned to send
func TestBlindRecipients_RefusesWhatItCannotSearch(t *testing.T) {

	pointed := map[string]any{"bcc": "https://example.com/@blind", "name": "p"}

	values := map[string]any{
		"struct":   hiddenObject{Name: "s", BCC: []string{"https://example.com/@blind"}},
		"pointer":  &pointed,
		"raw JSON": json.RawMessage(`{"bcc":"https://example.com/@blind","name":"r"}`),
		"channel":  make(chan int),
		"function": func() {},
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {

			result, err := StripBlindRecipients(map[string]any{"object": map[string]any{"value": value}})

			require.Error(t, err)
			require.Nil(t, result)
			require.NotContains(t, err.Error(), "@blind")
		})
	}

	// The originals keep their blind recipients
	require.Contains(t, pointed, "bcc")
}

// TestBlindRecipients_ContainsItself verifies that a document which holds itself is an error,
// and does not run forever
func TestBlindRecipients_ContainsItself(t *testing.T) {

	activity := map[string]any{"bcc": "https://example.com/@blind"}
	activity["object"] = activity

	result, err := StripBlindRecipients(activity)

	require.Error(t, err)
	require.Nil(t, result)
}

// FuzzBlindRecipients verifies that no JSON document keeps a blind property at any depth, and
// that the strip never changes its input
func FuzzBlindRecipients(f *testing.F) {

	f.Add(`{"bto":["a"],"object":{"bcc":"b","inReplyTo":{"bto":["c"]}}}`)
	f.Add(`{"items":[{"bcc":[]},[{"bto":1}]],"bto":null}`)
	f.Add(`{}`)

	f.Fuzz(func(t *testing.T, input string) {

		var document map[string]any

		if err := json.Unmarshal([]byte(input), &document); err != nil {
			t.Skip()
		}

		before := serialized(t, document)
		result := withoutBlindRecipients(t, document)

		require.False(t, hasBlindProperty(t, result))
		require.Equal(t, before, serialized(t, document))
	})
}
