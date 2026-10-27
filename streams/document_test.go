package streams

import (
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/require"
)

// TestDocument confirms that ID reads the "id" of a document built from a map[string]any.
func TestDocument(t *testing.T) {

	d := NewDocument(map[string]any{
		"id": "https://example.com",
	})

	require.Equal(t, "https://example.com", d.ID())
}

// TestDocumentMapOfAny confirms that a document built from a mapof.Any is a map, and that ID reads its "id".
func TestDocumentMapOfAny(t *testing.T) {

	d := NewDocument(mapof.Any{
		"id": "https://example.com",
	})

	require.True(t, d.IsMap())
	require.Equal(t, "https://example.com", d.ID())
}

// TestDocument_IsSameOrigin confirms that a document's id matches a URL only on the same scheme, host,
// and port, and that an id with no origin matches nothing.
func TestDocument_IsSameOrigin(t *testing.T) {

	const actor = "https://example.com/users/alice"

	test := func(id any, url string, expected bool) {
		document := NewDocument(map[string]any{vocab.PropertyID: id})
		require.Equal(t, expected, document.IsSameOrigin(url), "id %v, url %q", id, url)
	}

	test("https://example.com/notes/1", actor, true)
	test("https://EXAMPLE.com:443/notes/1", actor, true)
	test("https://example.com/notes/1#fragment", actor, true)
	test("http://example.com/notes/1", actor, false)
	test("https://example.com:8443/notes/1", actor, false)
	test("https://sub.example.com/notes/1", actor, false)
	test("https://example.com.evil.com/notes/1", actor, false)
	test("https://evil.com/notes/1", actor, false)
	test("urn:uuid:550e8400-e29b-41d4-a716-446655440000", actor, false)
	test("550e8400-e29b-41d4-a716-446655440000", actor, false)
	test("", actor, false)
	test(nil, actor, false)
	test("https://example.com/notes/1", "", false)
	test("", "", false)
}

// TestDocument_IsSameOrigin_BareLink confirms that a document holding only a URL compares that URL.
func TestDocument_IsSameOrigin_BareLink(t *testing.T) {
	require.True(t, NewDocument("https://example.com/notes/1").IsSameOrigin("https://example.com/users/alice"))
	require.False(t, NewDocument("https://evil.com/notes/1").IsSameOrigin("https://example.com/users/alice"))
}

// TestDocument_IsSameOrigin_IgnoresOriginProperty confirms that the ActivityStreams "origin" property
// plays no part in the comparison.
func TestDocument_IsSameOrigin_IgnoresOriginProperty(t *testing.T) {

	document := NewDocument(map[string]any{
		vocab.PropertyID:     "https://evil.com/activities/1",
		vocab.PropertyOrigin: "https://example.com/collections/1",
	})

	require.False(t, document.IsSameOrigin("https://example.com/users/alice"))
}
