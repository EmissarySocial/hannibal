package streams

import (
	"net/http"
	"testing"

	"github.com/benpate/hannibal/metadata"
	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/assert"
)

// TestDocument_Client confirms the Client accessor returns the client injected
// via WithClient.
func TestDocument_Client(t *testing.T) {

	client := testClient{}
	doc := NewDocument(map[string]any{}, WithClient(client))

	// A document always has a client, so only the injected one proves WithClient worked
	assert.Equal(t, client, doc.Client())
}

// TestDocument_SetValue confirms SetValue replaces the document's underlying value.
func TestDocument_SetValue(t *testing.T) {

	doc := NewDocument(map[string]any{vocab.PropertyID: "urn:original"})
	doc.SetValue(property.NewValue(map[string]any{vocab.PropertyID: "urn:replaced"}))

	assert.Equal(t, "urn:replaced", doc.ID())
}

// TestDocument_AddOptions confirms AddOptions applies options to a copy and
// returns it, leaving the original document's options unchanged.
func TestDocument_AddOptions(t *testing.T) {

	header := http.Header{}
	header.Set("Content-Type", vocab.ContentTypeActivityPub)

	original := NewDocument(map[string]any{})
	updated := original.AddOptions(WithHTTPHeader(header))

	assert.Equal(t, vocab.ContentTypeActivityPub, updated.HTTPHeader().Get("Content-Type"))
	assert.Empty(t, original.HTTPHeader().Get("Content-Type"))
}

// TestDocument_WithOptions confirms WithOptions applies options to the document
// in place.
func TestDocument_WithOptions(t *testing.T) {

	meta := metadata.Metadata{DocumentCategory: vocab.DocumentCategoryObject}

	doc := NewDocument(map[string]any{})
	doc.WithOptions(WithMetadata(meta))

	assert.Equal(t, vocab.DocumentCategoryObject, doc.Metadata.DocumentCategory)
}

// TestWithClient_NilFallsBackToDefault confirms passing a nil client installs
// the default client rather than leaving the document without one.
func TestWithClient_NilFallsBackToDefault(t *testing.T) {

	doc := NewDocument(map[string]any{}, WithClient(nil))
	assert.NotNil(t, doc.Client())
}

// TestNewCollection confirms the constructor sets the ID, type, and default context.
func TestNewCollection(t *testing.T) {

	collection := NewCollection("https://example.com/outbox")

	assert.Equal(t, "https://example.com/outbox", collection.ID)
	assert.Equal(t, vocab.CoreTypeCollection, collection.Type)
	assert.Equal(t, DefaultContext(), collection.Context)
}

// TestNewCollectionPage confirms the constructor sets the ID, type, and default context.
func TestNewCollectionPage(t *testing.T) {

	page := NewCollectionPage("https://example.com/outbox?page=1")

	assert.Equal(t, "https://example.com/outbox?page=1", page.ID)
	assert.Equal(t, vocab.CoreTypeCollectionPage, page.Type)
	assert.Equal(t, DefaultContext(), page.Context)
}

// TestDocument_RangeInReplyTo confirms the iterator loads the in-reply-to
// document and yields its addressees, and yields nothing when the property is absent.
func TestDocument_RangeInReplyTo(t *testing.T) {

	t.Run("yields addressees of the parent document", func(t *testing.T) {

		// The parent (in-reply-to) document is served by a mock client, keyed by URL.
		parentID := "https://example.com/notes/parent"
		client := testClient{data: mapof.Any{
			parentID: map[string]any{
				vocab.PropertyID: parentID,
				vocab.PropertyTo: "https://example.com/users/alice",
			},
		}}

		doc := NewDocument(map[string]any{
			vocab.PropertyInReplyTo: parentID,
		}, WithClient(client))

		var collected []string
		for address := range doc.RangeInReplyTo() {
			collected = append(collected, address)
		}

		assert.Contains(t, collected, "https://example.com/users/alice")
	})

	t.Run("yields the author (attributedTo) of the parent document", func(t *testing.T) {

		// The parent Note carries its author in `attributedTo` (not in an addressee field) — the
		// common shape for a reply target. RangeInReplyTo must still surface that author.
		parentID := "https://example.com/notes/parent"
		client := testClient{data: mapof.Any{
			parentID: map[string]any{
				vocab.PropertyID:           parentID,
				vocab.PropertyAttributedTo: "https://example.com/users/bob",
			},
		}}

		doc := NewDocument(map[string]any{
			vocab.PropertyInReplyTo: parentID,
		}, WithClient(client))

		var collected []string
		for address := range doc.RangeInReplyTo() {
			collected = append(collected, address)
		}

		assert.Contains(t, collected, "https://example.com/users/bob")
	})

	t.Run("no inReplyTo -> yields nothing", func(t *testing.T) {
		doc := NewDocument(map[string]any{})

		count := 0
		for range doc.RangeInReplyTo() {
			count++
		}
		assert.Equal(t, 0, count)
	})
}

// TestDocument_Load_InvalidURL confirms Load rejects a document whose ID is not
// a valid URL, and returns nil (no error) for an empty ID.
func TestDocument_Load_InvalidURL(t *testing.T) {

	t.Run("invalid URL -> error", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyID: "not-a-url"})
		_, err := doc.Load()
		assert.Error(t, err)
	})

	t.Run("empty ID -> nil document, no error", func(t *testing.T) {
		doc := NewDocument(map[string]any{})
		result, err := doc.Load()
		assert.NoError(t, err)
		assert.True(t, result.IsNil())
	})
}
