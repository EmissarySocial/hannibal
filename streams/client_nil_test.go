package streams

import (
	"encoding/json"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/require"
)

// TestNilClient_Load verifies that the client refuses every load, names the URL, and returns an empty Document
func TestNilClient_Load(t *testing.T) {

	for _, url := range []string{"https://example.org/users/alice", "", "not a url", "mailto:alice@example.org"} {
		t.Run(url, func(t *testing.T) {

			document, err := NewNilClient().Load(url, "an option", 7)

			require.Error(t, err)
			require.True(t, derp.IsInternalServerError(err))
			require.True(t, document.IsNil())
			require.Empty(t, document.ID())
		})
	}
}

// TestNilClient_SaveDeleteNoop verifies that the methods a cache would implement do nothing and succeed
func TestNilClient_SaveDeleteNoop(t *testing.T) {

	client := NewNilClient()

	require.NoError(t, client.Save(NewDocument(mapof.Any{vocab.PropertyID: "https://example.org/1"})))
	require.NoError(t, client.Delete("https://example.org/1"))
	require.NotPanics(t, func() { client.SetRootClient(nil) })
	require.NotPanics(t, func() { client.SetRootClient(client) })
}

// TestDocument_WithoutClient verifies every way of making a Document without a Client, and that
// each one can load nothing
func TestDocument_WithoutClient(t *testing.T) {

	var unmarshalled Document
	require.NoError(t, json.Unmarshal([]byte(`"https://example.org/users/alice"`), &unmarshalled))

	for name, document := range map[string]Document{
		"NewDocument":     NewDocument("https://example.org/users/alice"),
		"WithClient(nil)": NewDocument("https://example.org/users/alice", WithClient(nil)),
		"AddOptions":      NewDocument("https://example.org/users/alice").AddOptions(WithClient(nil)),
		"json.Unmarshal":  unmarshalled,
	} {
		t.Run(name, func(t *testing.T) {

			require.IsType(t, NilClient{}, document.getClient())

			// An explicit load reports the failure
			loaded, err := document.Load()
			require.Error(t, err)
			require.True(t, derp.IsInternalServerError(err))
			require.True(t, loaded.IsNil())

			// The loads that hide inside getters read as empty, and do not panic
			require.True(t, document.MustLoad().IsNil())
			require.True(t, document.LoadLink().IsNil())
			require.Equal(t, vocab.Unknown, document.Type())
			require.Empty(t, document.Name())
			require.True(t, document.Get(vocab.PropertyInbox).IsNil())

			// The id needs no load
			require.Equal(t, "https://example.org/users/alice", document.ID())
		})
	}
}

// TestDocument_NilDocumentHasNoClient verifies that the empty Document loads nothing either
func TestDocument_NilDocumentHasNoClient(t *testing.T) {

	nilDocument := NilDocument()
	require.IsType(t, NilClient{}, nilDocument.Client())

	emptyDocument := NewDocument(nil)
	require.IsType(t, NilClient{}, emptyDocument.Client())

	var zero Document
	require.Nil(t, zero.Client())
	require.IsType(t, NilClient{}, zero.getClient())
}

// TestDocument_WithoutClient_BareURLProperties verifies that a Document built from local data
// reads what it holds, and reads a bare URL as an id and nothing more
func TestDocument_WithoutClient_BareURLProperties(t *testing.T) {

	activity := NewDocument(mapof.Any{
		vocab.PropertyID:     "https://example.org/activities/1",
		vocab.PropertyType:   vocab.ActivityTypeLike,
		vocab.PropertyActor:  "https://example.org/users/alice",
		vocab.PropertyObject: "https://remote.example.org/notes/1",
		vocab.PropertyTo:     []any{"https://remote.example.org/users/bob"},
	})

	require.Equal(t, vocab.ActivityTypeLike, activity.Type())
	require.Equal(t, "https://example.org/users/alice", activity.ActorID())
	require.Equal(t, "https://remote.example.org/notes/1", activity.Object().ID())
	require.Equal(t, vocab.Unknown, activity.Object().Type())
	require.Empty(t, activity.Object().MediaType())
	require.True(t, activity.Actor().IsNil(), "Actor loads the actor, and there is no client to load it")

	// A property read from the Document inherits its lack of a client
	object := activity.Object()
	require.IsType(t, NilClient{}, object.Client())
}

// TestDocument_WithoutClient_ReplyParent verifies the one iterator that loads on its own: with no
// client it yields no one, and with a client it yields the author of the parent
func TestDocument_WithoutClient_ReplyParent(t *testing.T) {

	reply := mapof.Any{
		vocab.PropertyType:      vocab.ObjectTypeNote,
		vocab.PropertyInReplyTo: "https://remote.example.org/notes/1",
	}

	client := testClient{data: mapof.Any{
		"https://remote.example.org/notes/1": mapof.Any{
			vocab.PropertyID:           "https://remote.example.org/notes/1",
			vocab.PropertyType:         vocab.ObjectTypeNote,
			vocab.PropertyAttributedTo: "https://remote.example.org/users/bob",
		},
	}}

	withClient := make([]string, 0)

	for recipient := range NewDocument(reply, WithClient(client)).RangeInReplyTo() {
		withClient = append(withClient, recipient)
	}

	require.Contains(t, withClient, "https://remote.example.org/users/bob")

	for recipient := range NewDocument(reply).RangeInReplyTo() {
		t.Fatalf("a Document with no client yielded %q", recipient)
	}
}

// TestWithDefaultClient verifies that the option gives a Document the client that loads from the network
func TestWithDefaultClient(t *testing.T) {

	document := NewDocument("https://example.org/users/alice", WithDefaultClient())
	require.IsType(t, DefaultClient{}, document.Client())

	// The options are the client's own
	withOptions := NewDocument("https://example.org/users/alice", WithDefaultClient(allowPrivateIPs()))
	client, isDefault := withOptions.Client().(DefaultClient)
	require.True(t, isDefault)
	require.Len(t, client.options, 1)

	// A later option replaces it
	replaced := document.AddOptions(WithClient(nil))
	require.IsType(t, NilClient{}, replaced.Client())
}
