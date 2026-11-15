package collection

import (
	"net/http"
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/******************************************
 * ServeSummary
 ******************************************/

// TestServeSummary_PublishesCountNotMembers is the whole contract: the size is disclosed, the
// membership is not, and there is no affordance for a client to ask for the members either.
func TestServeSummary_PublishesCountNotMembers(t *testing.T) {

	ctx, recorder := newContext("https://example.com/followers")

	err := ServeSummary(recorder, ctx, "https://example.com/followers", 42)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := decodeBody(t, recorder)
	assert.Equal(t, vocab.ContextTypeActivityStreams, body[vocab.AtContext])
	assert.Equal(t, "https://example.com/followers", body[vocab.PropertyID])
	assert.Equal(t, vocab.CoreTypeOrderedCollection, body[vocab.PropertyType])
	assert.EqualValues(t, 42, body[vocab.PropertyTotalItems])

	// The absence of `first` is important -- it is what marks the collection as hidden
	// rather than empty. The rest must not leak members by any route.
	assert.NotContains(t, body, vocab.PropertyFirst)
	assert.NotContains(t, body, vocab.PropertyOrderedItems)
	assert.NotContains(t, body, vocab.PropertyItems)
	assert.NotContains(t, body, vocab.PropertyLast)
	assert.NotContains(t, body, vocab.PropertyNext)
}

// TestServeSummary_ZeroIsPublished is the regression that motivated this function. Marshalling a
// streams.OrderedCollection drops `totalItems` at zero (it is tagged `omitempty`), which makes an
// empty collection indistinguishable from one that reports nothing at all -- and a consumer that
// cannot tell "zero" from "unknown" will keep a previously-cached count forever.
func TestServeSummary_ZeroIsPublished(t *testing.T) {

	// An ActivityPub Accept header selects compact output, so the raw-bytes assertion below has a
	// single predictable spelling.
	ctx, recorder := newContextWithAccept("https://example.com/followers", vocab.ContentTypeActivityPub)

	err := ServeSummary(recorder, ctx, "https://example.com/followers", 0)
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	require.Contains(t, body, vocab.PropertyTotalItems, "totalItems must survive a zero count")
	assert.EqualValues(t, 0, body[vocab.PropertyTotalItems])

	// Checked at the byte level too: the decoded map cannot distinguish "key absent" from
	// "key present and zero" as bluntly as the wire format does.
	assert.Contains(t, recorder.Body.String(), `"totalItems":0`)
}

// TestServeSummary_ContentType confirms the response is labelled as ActivityPub.
func TestServeSummary_ContentType(t *testing.T) {

	ctx, recorder := newContext("https://example.com/followers")

	require.NoError(t, ServeSummary(recorder, ctx, "https://example.com/followers", 7))
	assert.Equal(t, vocab.ContentTypeActivityPub, recorder.Header().Get("Content-Type"))
}

// TestServeSummary_AppliesConfig confirms the functional options are honoured, so a summary
// collection can still carry attribution and audience like any other.
func TestServeSummary_AppliesConfig(t *testing.T) {

	ctx, recorder := newContext("https://example.com/followers")

	err := ServeSummary(recorder, ctx, "https://example.com/followers", 3,
		WithAttributedTo("https://example.com/@sarah"),
		WithAudience("https://example.com/followers"),
	)
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	assert.Equal(t, "https://example.com/@sarah", body[vocab.PropertyAttributedTo])
	assert.Equal(t, "https://example.com/followers", body[vocab.PropertyAudience])
	assert.EqualValues(t, 3, body[vocab.PropertyTotalItems])
}

// TestServeSummary_IgnoresPagingQuery confirms a summary never turns into a page. Serve() answers
// `?after=` with an OrderedCollectionPage; ServeSummary has no members to page through, so the
// parameter must not change the response.
func TestServeSummary_IgnoresPagingQuery(t *testing.T) {

	ctx, recorder := newContext("https://example.com/followers?after=FIRST")

	require.NoError(t, ServeSummary(recorder, ctx, "https://example.com/followers", 99))

	body := decodeBody(t, recorder)
	assert.Equal(t, vocab.CoreTypeOrderedCollection, body[vocab.PropertyType])
	assert.NotContains(t, body, vocab.PropertyOrderedItems)
	assert.NotContains(t, body, vocab.PropertyPartOf)
}

// TestServeSummary_ContentTypeNegotiation confirms the summary follows the same pretty/compact
// negotiation as Serve, so a browser gets readable output.
func TestServeSummary_ContentTypeNegotiation(t *testing.T) {

	t.Run("ActivityPub Accept -> compact", func(t *testing.T) {
		ctx, recorder := newContextWithAccept("https://example.com/followers", vocab.ContentTypeActivityPub)

		require.NoError(t, ServeSummary(recorder, ctx, "https://example.com/followers", 5))
		assert.NotContains(t, recorder.Body.String(), "\n    ")
	})

	t.Run("browser Accept -> pretty", func(t *testing.T) {
		ctx, recorder := newContextWithAccept("https://example.com/followers", "text/html")

		require.NoError(t, ServeSummary(recorder, ctx, "https://example.com/followers", 5))
		assert.Contains(t, recorder.Body.String(), "\n    ")
	})
}

/******************************************
 * ServeEmpty
 ******************************************/

// TestServeEmpty_PublishesZero is the bug fix. ServeEmpty previously marshalled a
// streams.OrderedCollection with TotalItems explicitly set to 0 -- and `omitempty` silently
// dropped it, so the one fact the function existed to state was the one it never sent.
func TestServeEmpty_PublishesZero(t *testing.T) {

	ctx, recorder := newContext("https://example.com/followers")

	err := ServeEmpty(recorder, ctx, "https://example.com/followers")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := decodeBody(t, recorder)
	require.Contains(t, body, vocab.PropertyTotalItems, "an empty collection must still report totalItems")
	assert.EqualValues(t, 0, body[vocab.PropertyTotalItems])

	assert.Equal(t, vocab.ContextTypeActivityStreams, body[vocab.AtContext])
	assert.Equal(t, "https://example.com/followers", body[vocab.PropertyID])
	assert.Equal(t, vocab.CoreTypeOrderedCollection, body[vocab.PropertyType])
	assert.NotContains(t, body, vocab.PropertyFirst)
	assert.NotContains(t, body, vocab.PropertyOrderedItems)
}

// TestServeEmpty_ContentType confirms the ActivityPub content type survived the rewrite -- the
// previous implementation set it by hand, and callers depend on it.
func TestServeEmpty_ContentType(t *testing.T) {

	ctx, recorder := newContext("https://example.com/followers")

	require.NoError(t, ServeEmpty(recorder, ctx, "https://example.com/followers"))
	assert.Equal(t, vocab.ContentTypeActivityPub, recorder.Header().Get("Content-Type"))
}
