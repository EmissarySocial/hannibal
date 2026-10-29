package hannibal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/require"
)

// TestIsActivityPubContentType confirms which single media types count as
// ActivityPub, ignoring parameters and case.
func TestIsActivityPubContentType(t *testing.T) {
	require.True(t, IsActivityPubContentType("application/json"))
	require.True(t, IsActivityPubContentType("application/json; everything after the semicolon is ignored"))
	require.True(t, IsActivityPubContentType("application/json; whocares=notme"))
	require.True(t, IsActivityPubContentType("application/activity+json"))
	require.True(t, IsActivityPubContentType("application/activity+json; charset=utf-8"))
	require.True(t, IsActivityPubContentType("APPLICATION/ACTIVITY+JSON"))
	require.True(t, IsActivityPubContentType("application/ld+json"))
	require.True(t, IsActivityPubContentType("application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""))

	require.False(t, IsActivityPubContentType("literally anything else"))
	require.False(t, IsActivityPubContentType("application/xml"))
	require.False(t, IsActivityPubContentType("application/xml; whocares=notme"))
	require.False(t, IsActivityPubContentType("image/webp"))

	// A LIST of media types is not a content type. This used to return TRUE by silently reading
	// only the first entry, which is how an "Accept" header could be mistaken for one.
	require.False(t, IsActivityPubContentType("application/activity+json, text/html"))
}

// TestAcceptsActivityPub covers the answer selected by a range of real "Accept" headers.
func TestAcceptsActivityPub(t *testing.T) {

	// Rows are named for the client they represent: the regressions worth catching are interop
	// regressions, not parser regressions.
	const as2Profile = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

	table := []struct {
		name   string
		accept string
		expect bool
	}{
		// Defaults. ActivityPub is never the answer to an unasked question.
		{"absent header", "", false},
		{"curl default", "*/*", false},
		{"type wildcard", "text/*", false},
		{"unrecognized only", "application/xml", false},
		{"unrelated type wildcard", "image/*", false},
		{"garbage", "this is not a media type", false},

		// Wildcards never select ActivityPub. Only the three exact media types do, so a client that
		// shrugs is not answered with an ActivityStreams document.
		{"application wildcard does not select activitypub", "application/*", false},
		{"leading wildcard wins the tie by client order", "*/*, application/activity+json", false},
		{"wildcard quality is honored", "text/*;q=1.0, application/activity+json;q=0.5", false},
		{"wildcard cannot rescue a refused representation", "text/html;q=0, */*", false},
		{"wildcard refusal does not override a specific accept", "*/*;q=0, text/html", false},

		// Browsers.
		{"chrome", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8", false},
		{"html only", "text/html", false},

		// Federated peers.
		{"mastodon", "application/activity+json, " + as2Profile, true},
		{"activity+json alone", vocab.ContentTypeActivityPub, true},
		{"ld+json with profile", as2Profile, true},
		{"ld+json bare", vocab.ContentTypeJSONLD, true},
		{"bare json", vocab.ContentTypeJSON, true},

		// The two situations the old predicate got wrong. These are the regression tests.
		{"q-values reorder the list", "text/html;q=0.9, application/activity+json;q=1.0", true},
		{"unsupported type listed first", "application/xrd+xml, application/activity+json", true},

		// Ordering at equal quality follows the order the client listed.
		{"html first at equal quality", "text/html, application/activity+json", false},
		{"activitypub first at equal quality", "application/activity+json, text/html", true},
		{"activitypub ahead of wildcard", "application/activity+json, */*", true},

		// Quality edge cases.
		{"html refused outright", "text/html;q=0, application/activity+json", true},
		{"everything refused", "text/html;q=0, application/activity+json;q=0", false},
		{"html wins on quality", "text/html;q=1.0, application/activity+json;q=0.9", false},
		{"malformed quality is discarded", "application/activity+json;q=bogus", false},
		{"malformed parameter still selects", "application/activity+json;;", true},

		// A quoted parameter value may contain a comma; splitting inside it would lose the entry.
		{"comma inside a quoted profile", `application/ld+json; profile="https://example.com/a,b"`, true},

		// Whitespace and casing, which clients are inconsistent about.
		{"padded entries", "  text/html ;  q=0.5 ,  application/activity+json  ", true},
		{"uppercase media type", "APPLICATION/ACTIVITY+JSON", true},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expect, acceptsActivityPub(test.accept), "Accept: %q", test.accept)
		})
	}
}

// TestParseAcceptEntry covers the entry parser's contract directly, including the refusals that
// acceptsActivityPub cannot see because a zero or negative quality never wins there.
func TestParseAcceptEntry(t *testing.T) {

	table := []struct {
		name          string
		text          string
		expectRange   string
		expectQuality float64
		expectOK      bool
	}{
		{"bare media type", "application/activity+json", "application/activity+json", 1, true},
		{"stated quality", "text/html;q=0.5", "text/html", 0.5, true},
		{"smallest positive quality", "text/html;q=0.001", "text/html", 0.001, true},
		{"quality above 1 is capped", "text/html;q=2", "text/html", 1, true},
		{"media type is lowercased", "TEXT/HTML", "text/html", 1, true},
		{"padding is trimmed", "  text/html ;  q=0.5 ", "text/html", 0.5, true},
		{"malformed parameter keeps the default quality", "application/activity+json;;", "application/activity+json", 1, true},

		// Refusals
		{"q=0 is a refusal", "text/html;q=0", "", 0, false},
		{"negative quality is a refusal", "text/html;q=-1", "", 0, false},
		{"unparseable quality", "text/html;q=bogus", "", 0, false},
		{"empty entry", "", "", 0, false},
		{"parameters only", ";q=1", "", 0, false},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			entry, ok := parseAcceptEntry(test.text)
			require.Equal(t, test.expectOK, ok, "entry: %q", test.text)
			require.Equal(t, test.expectRange, entry.mediaRange, "entry: %q", test.text)
			require.Equal(t, test.expectQuality, entry.quality, "entry: %q", test.text)
		})
	}
}

// TestSplitAcceptHeader covers the splitter's contract directly.
func TestSplitAcceptHeader(t *testing.T) {

	// acceptsActivityPub cannot see a mis-split: a torn fragment still yields a usable media type at
	// the default quality, so it ties with the entry it came from and resolves back to the same answer.
	table := []struct {
		name   string
		header string
		expect []string
	}{
		{"empty", "", []string{""}},
		{"single entry", "text/html", []string{"text/html"}},
		{"two entries", "text/html, application/activity+json", []string{"text/html", " application/activity+json"}},
		{"comma inside a quoted value", `application/ld+json; profile="a,b"`, []string{`application/ld+json; profile="a,b"`}},

		// An escaped quote must not close the quoted string -- otherwise the following comma reads
		// as a separator and the entry is torn in half.
		{"escaped quote then comma", `text/html; x="a\"b,c"`, []string{`text/html; x="a\"b,c"`}},

		// A backslash as the final byte has nothing to escape. This covers the bounds guard, which
		// would otherwise read past the end of the string.
		{"trailing backslash inside quotes", `text/html; x="a\`, []string{`text/html; x="a\`}},

		{"empty entries are preserved", ",,", []string{"", "", ""}},
		{"trailing comma", "text/html,", []string{"text/html", ""}},
	}

	for _, test := range table {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expect, splitAcceptHeader(test.header), "header: %q", test.header)
		})
	}
}

// TestIsActivityPubRequest confirms the request helpers read the "Accept" header.
func TestIsActivityPubRequest(t *testing.T) {

	t.Run("activitypub accept -> true", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
		request.Header.Set("Accept", vocab.ContentTypeActivityPub)
		require.True(t, IsActivityPubRequest(request))
		require.False(t, NotActivityPubRequest(request))
	})

	t.Run("html accept -> false", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
		request.Header.Set("Accept", "text/html")
		require.False(t, IsActivityPubRequest(request))
		require.True(t, NotActivityPubRequest(request))
	})

	t.Run("no accept header -> false", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
		require.False(t, IsActivityPubRequest(request))
		require.True(t, NotActivityPubRequest(request))
	})
}

// TestIsUndoableActivity confirms undoable activity types are recognized and
// non-undoable types (e.g. Create) are not.
func TestIsUndoableActivity(t *testing.T) {

	for _, activityType := range []string{
		vocab.ActivityTypeAnnounce,
		vocab.ActivityTypeDislike,
		vocab.ActivityTypeFollow,
		vocab.ActivityTypeLike,
		vocab.ActivityTypeBlock,
	} {
		require.True(t, IsUndoableActivity(activityType), activityType)
	}

	require.False(t, IsUndoableActivity(vocab.ActivityTypeCreate))
	require.False(t, IsUndoableActivity(vocab.ActivityTypeDelete))
	require.False(t, IsUndoableActivity("SomethingMadeUp"))
}
