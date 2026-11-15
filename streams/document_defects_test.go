package streams

import (
	"encoding/json"
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/require"
)

// TestDocument_ZeroValue verifies that a Document made without NewDocument reads as nil, and never panics
func TestDocument_ZeroValue(t *testing.T) {

	// A struct field that the remote JSON leaves out is a zero-value Document
	var holder struct {
		Actor Document `json:"actor"`
	}

	require.NoError(t, json.Unmarshal([]byte(`{}`), &holder))

	document := holder.Actor

	require.True(t, document.IsNil())
	require.False(t, document.NotNil())
	require.False(t, document.IsMap())
	require.False(t, document.IsString())
	require.Empty(t, document.ID())
	require.Equal(t, NilDocument().Type(), document.Type())
	require.Empty(t, document.Name())
	require.Empty(t, document.Summary())
	require.Empty(t, document.SummaryWithTagLinks())
	require.Empty(t, document.String())
	require.Zero(t, document.Len())
	require.True(t, document.Get(vocab.PropertyObject).IsNil())
	require.True(t, document.Head().IsNil())
	require.True(t, document.Tail().IsNil())
	require.Empty(t, document.Map())
	require.Nil(t, document.Value())
	require.True(t, document.Clone().IsNil())
	require.Empty(t, document.SliceOfString())

	for range document.Range() {
		t.Fatal("a nil document has nothing to range over")
	}

	encoded, err := json.Marshal(document)
	require.NoError(t, err)
	require.Equal(t, "null", string(encoded))
}

// TestDocument_SettersWriteWhatTheyReport verifies each setter on each shape of document
func TestDocument_SettersWriteWhatTheyReport(t *testing.T) {

	t.Run("a nil document becomes an object", func(t *testing.T) {

		document := NilDocument()
		require.True(t, document.SetString(vocab.PropertyName, "Alice"))
		require.Equal(t, "Alice", document.Name())

		var zero Document
		require.True(t, zero.SetID("https://example.com/1"))
		require.Equal(t, "https://example.com/1", zero.ID())
	})

	t.Run("a list keeps every item after the first", func(t *testing.T) {

		document := NewDocument([]any{
			map[string]any{"id": "https://example.com/1"},
			map[string]any{"id": "https://example.com/2"},
			"https://example.com/3",
		})

		require.True(t, document.SetString(vocab.PropertyName, "First"))
		require.Equal(t, 3, document.Len())
		require.Equal(t, "First", document.Head().Name())
		require.Equal(t, "https://example.com/2", document.Tail().Head().ID())
	})

	t.Run("a string document becomes an object that keeps its id", func(t *testing.T) {

		document := NewDocument("https://example.com/notes/1")

		require.True(t, document.SetTo("https://example.com/@alice"))
		require.True(t, document.SetCC("https://example.com/@bob", "https://example.com/@carol"))
		require.True(t, document.AppendTo("https://example.com/@dave"))
		require.True(t, document.AppendBCC("https://example.com/@erin"))
		document.SetProperty(vocab.PropertyName, "Note")

		require.Equal(t, "https://example.com/notes/1", document.ID())
		require.Equal(t, "Note", document.Name())
		require.Equal(t, []string{"https://example.com/@alice", "https://example.com/@dave"}, []string(document.To().SliceOfString()))
		require.Equal(t, []string{"https://example.com/@bob", "https://example.com/@carol"}, []string(document.CC().SliceOfString()))
		require.Equal(t, []string{"https://example.com/@erin"}, []string(document.BCC().SliceOfString()))
	})

	t.Run("an object is changed in place, as before", func(t *testing.T) {

		source := map[string]any{"id": "https://example.com/1"}
		document := NewDocument(source)

		require.True(t, document.SetID("https://example.com/2"))
		require.True(t, document.SetBTo("https://example.com/@alice"))
		require.Equal(t, "https://example.com/2", source["id"])
		require.Equal(t, "https://example.com/@alice", source["bto"])
	})
}

// TestOrderedCollection_ReadsOrderedItems verifies that both ordered types read `orderedItems`,
// accept `items` from a server that writes the wrong name, and keep their id
func TestOrderedCollection_ReadsOrderedItems(t *testing.T) {

	var collection OrderedCollection
	require.NoError(t, json.Unmarshal([]byte(`{"id":"https://example.com/outbox","type":"OrderedCollection","totalItems":2,"orderedItems":["https://example.com/1","https://example.com/2"]}`), &collection))
	require.Equal(t, "https://example.com/outbox", collection.ID)
	require.Len(t, collection.OrderedItems, 2)
	require.Equal(t, "https://example.com/1", collection.OrderedItems[0])

	var page OrderedCollectionPage
	require.NoError(t, json.Unmarshal([]byte(`{"id":"https://example.com/outbox?page=1","type":"OrderedCollectionPage","orderedItems":[{"id":"https://example.com/1"}]}`), &page))
	require.Len(t, page.OrderedItems, 1)

	// The correct property wins when both are present
	var both OrderedCollection
	require.NoError(t, both.UnmarshalMap(mapof.Any{"type": "OrderedCollection", "orderedItems": []any{"right"}, "items": []any{"wrong", "wrong"}}))
	require.Equal(t, []any{"right"}, both.OrderedItems)

	var misnamed OrderedCollection
	require.NoError(t, misnamed.UnmarshalMap(mapof.Any{"type": "OrderedCollection", "items": []any{"only"}}))
	require.Equal(t, []any{"only"}, misnamed.OrderedItems)

	var plain Collection
	require.NoError(t, json.Unmarshal([]byte(`{"id":"https://example.com/likes","type":"Collection","items":["https://example.com/1"]}`), &plain))
	require.Equal(t, "https://example.com/likes", plain.ID)
	require.Len(t, plain.Items, 1)
}

// TestContext_UnmarshalJSON verifies that every form of @context can be read, and that the
// package reads what it writes
func TestContext_UnmarshalJSON(t *testing.T) {

	var single Context
	require.NoError(t, json.Unmarshal([]byte(`"https://www.w3.org/ns/activitystreams"`), &single))
	require.Len(t, single, 1)
	require.Equal(t, vocab.NamespaceActivityStreams, single[0].Vocabulary)

	var list Context
	require.NoError(t, json.Unmarshal([]byte(`["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1",{"@language":"en","toot":"http://joinmastodon.org/ns#","featured":{"@id":"toot:featured","@type":"@id"}}]`), &list))
	require.Len(t, list, 3)
	require.Equal(t, "https://w3id.org/security/v1", list[1].Vocabulary)
	require.Equal(t, "en", list[2].Language)
	require.Equal(t, "http://joinmastodon.org/ns#", list[2].Extensions["toot"])
	require.NotContains(t, list[2].Extensions, "featured", "a term that is not a string is skipped")

	var object Context
	require.NoError(t, json.Unmarshal([]byte(`{"@vocab":"https://www.w3.org/ns/activitystreams","@language":"fr"}`), &object))
	require.Len(t, object, 1)

	for _, entry := range object {
		require.Equal(t, "fr", entry.Language)
	}

	var empty Context
	require.NoError(t, json.Unmarshal([]byte(`null`), &empty))
	require.Empty(t, empty)

	var wrong Context
	require.Error(t, json.Unmarshal([]byte(`[5]`), &wrong))
	require.Error(t, json.Unmarshal([]byte(`7`), &wrong))

	// Round trip: what the package writes, it reads
	original := NewContext(vocab.NamespaceActivityStreams, "https://w3id.org/security/v1")
	original.Add("https://example.com/ns").WithLanguage("de").WithExtension("ex", "https://example.com/ns#")

	encoded, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Context
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, original, decoded)

	// The header type that carries a context can be read from a normal document
	var header CollectionHeader
	require.NoError(t, json.Unmarshal([]byte(`{"@context":"https://www.w3.org/ns/activitystreams","id":"https://example.com/outbox","type":"OrderedCollection"}`), &header))
}
