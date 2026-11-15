package streams

import (
	"encoding/json"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocument_MarshalJSON confirms a document marshals to the JSON of its
// underlying value, not the wrapper struct.
func TestDocument_MarshalJSON(t *testing.T) {

	doc := NewDocument(map[string]any{vocab.PropertyID: "https://example.com/1"})

	output, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"https://example.com/1"}`, string(output))
}

// TestDocument_UnmarshalJSON confirms a document unmarshals JSON into its value
// and the accessors then read it back.
func TestDocument_UnmarshalJSON(t *testing.T) {

	doc := NewDocument(nil)
	err := json.Unmarshal([]byte(`{"id":"https://example.com/2","name":"Alice"}`), &doc)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/2", doc.ID())
	assert.Equal(t, "Alice", doc.Name())
}

// TestDocument_UnmarshalJSON_CannotSetNoStore confirms that a remote document naming the NoStore
// policy, at the top level or inside a "metadata" object, never sets it.
func TestDocument_UnmarshalJSON_CannotSetNoStore(t *testing.T) {

	doc := NewDocument(nil)
	err := json.Unmarshal([]byte(`{"id":"https://example.com/2","NoStore":true,"noStore":true,"Metadata":{"NoStore":true},"metadata":{"noStore":true}}`), &doc)

	require.NoError(t, err)
	assert.False(t, doc.Metadata.NoStore)
}

// TestDocument_MarshalJSON_OmitsNoStore confirms that a document marked NoStore marshals without it.
func TestDocument_MarshalJSON_OmitsNoStore(t *testing.T) {

	doc := NewDocument(map[string]any{vocab.PropertyID: "https://example.com/1"})
	doc.Metadata.NoStore = true

	output, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"https://example.com/1"}`, string(output))
}

// TestDocument_UnmarshalJSON_ZeroValue confirms the idiomatic
// `var d Document; json.Unmarshal(data, &d)` works without panicking. A
// zero-value Document has a nil value interface; UnmarshalJSON must guard for it.
func TestDocument_UnmarshalJSON_ZeroValue(t *testing.T) {

	var doc Document
	err := json.Unmarshal([]byte(`{"id":"https://example.com/9","name":"Bob"}`), &doc)

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/9", doc.ID())
	assert.Equal(t, "Bob", doc.Name())
}

// TestDocument_UnmarshalJSON_Invalid confirms malformed JSON returns an error
// rather than panicking.
func TestDocument_UnmarshalJSON_Invalid(t *testing.T) {

	var doc Document
	err := json.Unmarshal([]byte(`{not valid json`), &doc)
	assert.Error(t, err)
}

// TestDocument_JSON_RoundTrip confirms a document survives a marshal/unmarshal
// round trip unchanged.
func TestDocument_JSON_RoundTrip(t *testing.T) {

	original := `{"id":"https://example.com/3","type":"Note","name":"Test"}`

	var doc Document
	require.NoError(t, json.Unmarshal([]byte(original), &doc))

	output, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.JSONEq(t, original, string(output))
}

// TestParseDocument verifies every form of JSON that a Document can hold, and that the result
// reads the same as one made by NewDocument
func TestParseDocument(t *testing.T) {

	for name, test := range map[string]struct {
		json     string
		expected any
	}{
		"object":       {`{"id":"https://example.com/1","type":"Note"}`, map[string]any{"id": "https://example.com/1", "type": "Note"}},
		"empty object": {`{}`, map[string]any{}},
		"list":         {`["https://example.com/1",{"id":"https://example.com/2"}]`, []any{"https://example.com/1", map[string]any{"id": "https://example.com/2"}}},
		"string":       {`"https://example.com/1"`, "https://example.com/1"},
		"number":       {`42`, float64(42)},
		"boolean":      {`true`, true},
		"whitespace":   {" \n {\"type\":\"Note\"} \n", map[string]any{"type": "Note"}},
	} {
		t.Run(name, func(t *testing.T) {

			document, err := ParseDocument([]byte(test.json))
			require.NoError(t, err)

			expected := NewDocument(test.expected)
			require.Equal(t, expected.Value(), document.Value())
			require.Equal(t, expected.ID(), document.ID())
			require.Equal(t, expected.Type(), document.Type())
			require.IsType(t, NilClient{}, document.getClient())
		})
	}

	// JSON null is a Document that holds nothing
	nothing, err := ParseDocument([]byte(`null`))
	require.NoError(t, err)
	require.True(t, nothing.IsNil())
}

// TestParseDocument_Invalid verifies that anything that is not JSON is an error with no
// status of its own, and that the Document returned with it is empty
func TestParseDocument_Invalid(t *testing.T) {

	for name, data := range map[string][]byte{
		"empty":           {},
		"nil":             nil,
		"html":            []byte(`<html>Tombstone</html>`),
		"truncated":       []byte(`{"type":"Note"`),
		"trailing data":   []byte(`{"type":"Note"} {"type":"Note"}`),
		"single quotes":   []byte(`{'type':'Note'}`),
		"invalid unicode": {'"', 0xff, '"', '{'},
	} {
		t.Run(name, func(t *testing.T) {

			document, err := ParseDocument(data)
			require.Error(t, err)
			require.True(t, document.IsNil())
			require.Equal(t, "hannibal.streams.ParseDocument", derp.Location(err))
		})
	}
}

// TestParseDocument_Options verifies that the options apply to the parsed Document, as they
// do for NewDocument
func TestParseDocument_Options(t *testing.T) {

	client := newRecordingClient()

	document, err := ParseDocument([]byte(`{"id":"https://example.com/1","actor":"https://example.com/users/alice"}`), WithClient(client), WithNoStore())
	require.NoError(t, err)
	require.Same(t, client, document.Client())
	require.True(t, document.Metadata.NoStore)

	// A property inherits the client
	actor := document.Get("actor")
	require.Same(t, client, actor.Client())
}

// FuzzParseDocument verifies that no input makes the parser panic, and that a Document it
// accepts can be written back as JSON and parsed to the same value
func FuzzParseDocument(f *testing.F) {

	f.Add([]byte(`{"id":"https://example.com/1","type":"Note"}`))
	f.Add([]byte(`["a",{"b":[1,2,{"c":null}]}]`))
	f.Add([]byte(`"https://example.com/1"`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"type":"Note"`))
	f.Add([]byte{0xff, 0xfe})

	f.Fuzz(func(t *testing.T, data []byte) {

		document, err := ParseDocument(data)

		if err != nil {
			require.True(t, document.IsNil())
			return
		}

		written, err := json.Marshal(document)
		require.NoError(t, err)

		again, err := ParseDocument(written)
		require.NoError(t, err)
		require.Equal(t, document.Value(), again.Value())
	})
}
