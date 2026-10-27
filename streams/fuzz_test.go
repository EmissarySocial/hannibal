package streams

import (
	"encoding/json"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/metadata"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/uri"
	"github.com/stretchr/testify/require"
)

// FuzzDocumentUnmarshalJSON parses arbitrary bytes into a Document and then walks its
// accessors, confirming that neither parsing nor traversal panics on hostile input.
func FuzzDocumentUnmarshalJSON(f *testing.F) {

	seedJSON(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		document := NilDocument()

		// A parse error is an acceptable outcome; we only require no panic.
		if err := json.Unmarshal(data, &document); err != nil {
			return
		}

		walkDocument(t, document)
	})
}

// FuzzCollectionUnmarshalJSON ensures Collection.UnmarshalJSON never panics on arbitrary input.
func FuzzCollectionUnmarshalJSON(f *testing.F) {

	seedJSON(f)

	f.Fuzz(func(_ *testing.T, data []byte) {
		var collection Collection
		_ = json.Unmarshal(data, &collection)
	})
}

// FuzzOrderedCollectionUnmarshalJSON ensures OrderedCollection.UnmarshalJSON never panics.
func FuzzOrderedCollectionUnmarshalJSON(f *testing.F) {

	seedJSON(f)

	f.Fuzz(func(_ *testing.T, data []byte) {
		var collection OrderedCollection
		_ = json.Unmarshal(data, &collection)
	})
}

// FuzzCollectionPageUnmarshalJSON ensures CollectionPage.UnmarshalJSON never panics.
func FuzzCollectionPageUnmarshalJSON(f *testing.F) {

	seedJSON(f)

	f.Fuzz(func(_ *testing.T, data []byte) {
		var page CollectionPage
		_ = json.Unmarshal(data, &page)
	})
}

// FuzzOrderedCollectionPageUnmarshalJSON ensures OrderedCollectionPage.UnmarshalJSON never panics.
func FuzzOrderedCollectionPageUnmarshalJSON(f *testing.F) {

	seedJSON(f)

	f.Fuzz(func(_ *testing.T, data []byte) {
		var page OrderedCollectionPage
		_ = json.Unmarshal(data, &page)
	})
}

// FuzzContextUnmarshalJSON ensures Context.UnmarshalJSON never panics. The custom decoder
// branches on the first byte (string, object, or array), so it must tolerate empty and
// malformed input safely.
func FuzzContextUnmarshalJSON(f *testing.F) {

	f.Add([]byte(``))
	f.Add([]byte(`"https://www.w3.org/ns/activitystreams"`))
	f.Add([]byte(`{"@vocab":"x","@language":"en"}`))
	f.Add([]byte(`["a",{"b":"c"}]`))
	f.Add([]byte(`[`))
	f.Add([]byte(`{`))
	f.Add([]byte(`123`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		var context Context
		_ = json.Unmarshal(data, &context)
	})
}

// FuzzDocumentMetadataStaysInternal confirms that no remote payload can set a document's metadata,
// and that no metadata, NoStore and Labels included, ever reaches the JSON a document marshals to.
func FuzzDocumentMetadataStaysInternal(f *testing.F) {

	seedJSON(f)

	// Hostile payloads that name the internal fields, in every spelling a decoder might honor
	f.Add([]byte(`{"id":"https://x/1","NoStore":true,"noStore":true,"nostore":true,"no_store":true}`))
	f.Add([]byte(`{"Metadata":{"NoStore":true,"Labels":[{"Value":"x","IsHidden":true}]}}`))
	f.Add([]byte(`{"metadata":{"noStore":true,"labels":[{"value":"x","isHidden":true}],"hashedId":"abc"}}`))
	f.Add([]byte(`{"HashedID":"abc","DocumentCategory":"Actor","RelationType":"Reply","RelationHref":"https://x/2","Replies":9}`))
	f.Add([]byte(`[{"NoStore":true},{"Metadata":{"NoStore":true}}]`))

	f.Fuzz(func(t *testing.T, data []byte) {

		document := NilDocument()

		if err := json.Unmarshal(data, &document); err != nil {
			return
		}

		// The parser fills only the value, never Metadata
		require.Zero(t, document.Metadata, "metadata set from %q", data)

		// Marking the document server-side changes nothing on the wire
		plain, err := json.Marshal(document)
		require.NoError(t, err)

		marked, err := json.Marshal(document.AddOptions(
			WithNoStore(),
			WithLabels(metadata.LabelSet{{Value: "Muted", IsHidden: true}}),
			WithRelation(vocab.RelationTypeReply, "https://example.com/parent"),
			WithDocumentCategory(vocab.ObjectTypeNote),
		))
		require.NoError(t, err)
		require.JSONEq(t, string(plain), string(marked))
	})
}

// FuzzOptionsClientLoad confirms that an OptionsClient passes any URL through untouched, with its
// bound options first, and that no URL a remote server supplies can change what it binds.
func FuzzOptionsClientLoad(f *testing.F) {

	f.Add("https://example.com/note")
	f.Add("https://example.com/note#main-key")
	f.Add("https://example.com/missing")
	f.Add("")
	f.Add("\x00\xff not a url")
	f.Add("https://example.com/note/" + string(make([]byte, 4096)))

	f.Fuzz(func(t *testing.T, url string) {

		inner := newRecordingClient()
		client := NewOptionsClient(inner, "bound")

		result, err := client.Load(url, "call")
		require.Equal(t, []any{"bound", "call"}, inner.optionsFor(url))

		if err != nil {
			require.True(t, derp.IsNotFound(err), "unexpected error for %q: %v", url, err)
			return
		}

		// ID() sanitizes its value as HTML, so compare the raw id the inner client returned
		require.Equal(t, client, result.Client())
		require.Equal(t, url, result.Get(vocab.PropertyID).rawString())
	})
}

// FuzzDocumentIsSameOrigin confirms that IsSameOrigin never panics, never matches an id with no
// origin, and gives the same answer with its two sides swapped.
func FuzzDocumentIsSameOrigin(f *testing.F) {

	f.Add("https://example.com/notes/1", "https://example.com/users/alice")
	f.Add("https://example.com/notes/1", "https://evil.com/users/alice")
	f.Add("urn:uuid:550e8400-e29b-41d4-a716-446655440000", "https://example.com/")
	f.Add("https://user@example.com/a", "https://example.com/a")
	f.Add("https://example.com<!-- -->/a", "https://example.com/a")
	f.Add("", "")

	f.Fuzz(func(t *testing.T, id string, url string) {

		if !NewDocument(map[string]any{vocab.PropertyID: id}).IsSameOrigin(url) {
			return
		}

		// A match needs an origin on both sides, and holds in either direction
		require.NotEmpty(t, uri.Host(url))
		require.True(t, NewDocument(map[string]any{vocab.PropertyID: url}).IsSameOrigin(NewDocument(map[string]any{vocab.PropertyID: id}).ID()))
	})
}

// FuzzInteractionPolicyUnmarshalJSON parses arbitrary bytes into an InteractionPolicy, and confirms that a
// successful parse re-encodes stably and answers every permission without panicking or breaking GoToSocial's rules
func FuzzInteractionPolicyUnmarshalJSON(f *testing.F) {

	seedInteractionPolicyJSON(f)

	f.Fuzz(func(t *testing.T, data []byte, actorID string) {

		policy := InteractionPolicy{}

		// A parse error is an acceptable outcome; it must leave nothing half-decoded
		if err := json.Unmarshal(data, &policy); err != nil {
			return
		}

		// Re-encoding is stable: encode, decode, encode gives the same bytes
		encoded, err := json.Marshal(policy)
		require.NoError(t, err)

		again := InteractionPolicy{}
		require.NoError(t, json.Unmarshal(encoded, &again))

		reencoded, err := json.Marshal(again)
		require.NoError(t, err)
		require.JSONEq(t, string(encoded), string(reencoded))

		// Every permission answers with a known result, and the fixed rules always hold
		const authorID = "https://a.example/u/a"
		request := InteractionRequest{
			ActorID:   actorID,
			AuthorID:  authorID,
			Mentioned: []string{actorID},
			IsMember:  func(collectionID string) bool { return len(collectionID)%2 == 0 },
		}

		for _, permission := range []string{vocab.PropertyCanLike, vocab.PropertyCanReply, vocab.PropertyCanAnnounce, vocab.PropertyCanQuote} {

			result := policy.CanInteract(permission, request)
			require.Contains(t, []InteractionResult{InteractionDenied, InteractionNeedsApproval, InteractionAllowed}, result)

			if actorID == "" {
				require.Equal(t, InteractionDenied, result, "an anonymous actor is always denied")
			}

			author := request
			author.ActorID = authorID
			require.Equal(t, InteractionAllowed, policy.CanInteract(permission, author), "the author is always allowed")

			_ = policy.Defines(permission)
			_ = policy.AutomaticApproval(permission)
			_ = policy.ManualApproval(permission)
		}

		require.Equal(t, policy.IsNil(), !policy.NotNil())
		_ = policy.ID()
	})
}

// FuzzDocumentInteractionAccessors reads the interaction and quote accessors of an arbitrary document, and
// confirms that none of them panics, and that reading a map's interactionPolicy never loads anything
func FuzzDocumentInteractionAccessors(f *testing.F) {

	seedJSON(f)
	f.Add([]byte(`{"interactionPolicy":"https://example.com/note/policy","quote":"https://example.com/note/q"}`))
	f.Add([]byte(`{"interactionPolicy":[{"canReply":{}}],"quoteUrl":["a","b"],"_misskey_quote":{"id":"c"}}`))
	f.Add([]byte(`{"quoteAuthorization":{"type":"QuoteAuthorization","interactingObject":"a","interactionTarget":{"id":"b"}}}`))

	f.Fuzz(func(t *testing.T, data []byte) {

		var value any

		if err := json.Unmarshal(data, &value); err != nil {
			return
		}

		client := newRecordingClient()
		document := NewDocument(value, WithClient(client))

		// RULE: The policy is read, never loaded
		policy := document.InteractionPolicy()

		if document.IsMap() {
			client.mutex.Lock()
			loads := len(client.options)
			client.mutex.Unlock()
			require.Zero(t, loads, "reading interactionPolicy loaded a document")
		}

		_ = policy.CanInteract(vocab.PropertyCanReply, InteractionRequest{ActorID: "https://b.example/u/s"})
		_ = document.Quote().ID()
		_ = document.QuoteAuthorization().InteractingObject().ID()
		_ = document.QuoteAuthorization().InteractionTarget().ID()
	})
}

// seedJSON returns a corpus of valid, malformed, and hostile JSON payloads shared
// by the streams fuzz targets. ActivityPub documents arrive from untrusted remote
// servers, so every parser below must survive arbitrary bytes without panicking.
func seedJSON(f *testing.F) {
	f.Add([]byte(``))
	f.Add([]byte(`null`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`""`))
	f.Add([]byte(`0`))
	f.Add([]byte(`"a string"`))
	f.Add([]byte(`{"type":"Note","content":"hello","id":"https://x/1"}`))
	f.Add([]byte(`{"type":"Create","actor":"https://x/@me","object":{"type":"Note"}}`))
	f.Add([]byte(`{"type":"Collection","totalItems":2,"items":[{"id":"a"},{"id":"b"}]}`))
	f.Add([]byte(`{"type":"OrderedCollection","orderedItems":["a","b"]}`))
	f.Add([]byte(`{"@context":["https://www.w3.org/ns/activitystreams",{"toot":"x"}]}`))
	f.Add([]byte(`{"@context":"https://www.w3.org/ns/activitystreams"}`))
	f.Add([]byte(`{"to":["a","b"],"cc":"c","tag":[{"type":"Mention","href":"h"}]}`))
	f.Add([]byte(`{"items":{"id":"single-not-array"}}`))
	f.Add([]byte(`{"height":"not-a-number","width":1.5}`))
	f.Add([]byte(`{` + `"a":` + `"unterminated`))            // malformed
	f.Add([]byte(`{"deeply":{"nested":{"value":[[[1]]]}}}`)) // nesting
}

// walkDocument exercises the accessor surface of a parsed Document so that the fuzzer
// reaches code beyond the unmarshaler itself. None of these calls may panic, no matter
// what was parsed.
func walkDocument(t *testing.T, doc Document) {
	t.Helper()

	_ = doc.Type()
	_ = doc.ID()
	_ = doc.IsNil()
	_ = doc.IsString()
	_ = doc.IsMap()
	_ = doc.IsSlice()
	_ = doc.Slice()
	_ = doc.Actor().ID()
	_ = doc.Object().Type()
	_ = doc.Content()
	_ = doc.Summary()
	_ = doc.HTMLString()
	_ = doc.Height()
	_ = doc.Published()

	// Walk the addressee and item iterators, which traverse nested values.
	for range doc.RangeAddressees() {
	}
	for range doc.Range() {
	}
}

// seedInteractionPolicyJSON adds interactionPolicy shapes, real and hostile, to a fuzz corpus
func seedInteractionPolicyJSON(f *testing.F) {
	f.Add([]byte(`{"canReply":{"automaticApproval":["https://www.w3.org/ns/activitystreams#Public"]}}`), "https://b.example/u/s")
	f.Add([]byte(`{"canReply":{"automaticApproval":["https://a.example/u/a"],"manualApproval":["as:Public"]}}`), "https://b.example/u/s")
	f.Add([]byte(`{"canLike":{"always":["https://a.example/u/a/followers"],"approvalRequired":"Public"}}`), "https://b.example/u/s")
	f.Add([]byte(`{"canQuote":{"automaticApproval":[{"id":"https://b.example/u/s"},{"@id":"x"},{"type":"Link"}]}}`), "https://b.example/u/s")
	f.Add([]byte(`{"canReply":{"automaticApproval":null,"always":[[["deep"]]]},"canAnnounce":[]}`), "")
	f.Add([]byte(`[{"canReply":"https://a.example/policy"},7]`), "https://a.example/u/a")
	f.Add([]byte(`"https://a.example/policy"`), "https://b.example/u/s")
	f.Add([]byte(`{"canReply":{"automaticApproval":{"id":{"id":{"id":"x"}}}}}`), "x")
	f.Add([]byte(`null`), "")
	f.Add([]byte(`{`), "")
}
