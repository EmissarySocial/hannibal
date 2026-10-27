package collections

import (
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
)

// mapClient is an in-memory streams.Client that resolves URLs from a fixed map,
// so paging tests can follow "next" links without touching the network.
type mapClient struct {
	documents map[string]map[string]any
}

func (client mapClient) SetRootClient(streams.Client) {}

func (client mapClient) Load(uri string, _ ...any) (streams.Document, error) {
	if value, ok := client.documents[uri]; ok {
		return streams.NewDocument(value, streams.WithClient(client)), nil
	}
	return streams.NilDocument(), derp.Internal("collections.mapClient.Load", "Unknown URI", uri)
}

func (client mapClient) Save(streams.Document) error { return nil }
func (client mapClient) Delete(_ string) error       { return nil }

// TestRangePages_SinglePage confirms a single inline collection yields exactly
// one page (itself), with no infinite looping.
func TestRangePages_SinglePage(t *testing.T) {

	collection := inlineCollection("https://example.com/1", "https://example.com/2")

	pages := 0
	for page := range RangePages(collection) {
		pages++
		assert.Equal(t, 2, page.Items().Len())
	}

	assert.Equal(t, 1, pages)
}

// TestRangePages_InlineFirstPage reproduces the Mastodon "replies" shape: a
// Collection header whose "first" is an INLINE CollectionPage with no "id" and
// empty items, pointing via "next" to the real page that holds the items. The
// inline first page must be used as-is (not Load()ed away to Nil), so traversal
// follows "next" and reaches the items instead of stopping early.
func TestRangePages_InlineFirstPage(t *testing.T) {

	const nextURL = "https://example.com/replies?page=true"

	client := mapClient{documents: map[string]map[string]any{
		nextURL: {
			vocab.PropertyID:   nextURL,
			vocab.PropertyType: vocab.CoreTypeCollectionPage,
			vocab.PropertyItems: []any{
				map[string]any{vocab.PropertyID: "https://example.com/reply/1"},
				map[string]any{vocab.PropertyID: "https://example.com/reply/2"},
			},
		},
	}}

	collection := streams.NewDocument(map[string]any{
		vocab.PropertyID:   "https://example.com/replies",
		vocab.PropertyType: vocab.CoreTypeCollection,
		vocab.PropertyFirst: map[string]any{
			vocab.PropertyType:  vocab.CoreTypeCollectionPage,
			vocab.PropertyNext:  nextURL,
			vocab.PropertyItems: []any{}, // empty inline first page
		},
	}, streams.WithClient(client))

	got := collectIDs(RangeDocuments(collection))
	assert.Equal(t, []string{
		"https://example.com/reply/1",
		"https://example.com/reply/2",
	}, got)
}

// TestRangePages_Empty confirms a Nil collection yields no pages.
func TestRangePages_Empty(t *testing.T) {

	pages := 0
	for range RangePages(streams.NilDocument()) {
		pages++
	}

	assert.Equal(t, 0, pages)
}

// TestRangePages_EarlyStop confirms RangePages honors an early break from the
// consumer.
func TestRangePages_EarlyStop(t *testing.T) {

	collection := inlineCollection("https://example.com/1")

	pages := 0
	for range RangePages(collection) {
		pages++
		break
	}

	assert.Equal(t, 1, pages)
}

// TestRangeDocuments_SinglePage confirms RangeDocuments yields each item in the
// page, in order.
func TestRangeDocuments_SinglePage(t *testing.T) {

	collection := inlineCollection(
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	)

	got := collectIDs(RangeDocuments(collection))
	assert.Equal(t, []string{
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	}, got)
}

// TestRangeDocuments_Empty confirms an empty collection yields no documents.
func TestRangeDocuments_Empty(t *testing.T) {
	assert.Empty(t, collectIDs(RangeDocuments(streams.NilDocument())))
}

// TestRangeDocuments_EarlyStop confirms RangeDocuments honors an early break.
func TestRangeDocuments_EarlyStop(t *testing.T) {

	collection := inlineCollection(
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	)

	var got []string
	for document := range RangeDocuments(collection) {
		got = append(got, document.ID())
		if document.ID() == "https://example.com/2" {
			break
		}
	}

	assert.Equal(t, []string{"https://example.com/1", "https://example.com/2"}, got)
}

// TestRangePages_NextCycle confirms the default page cap terminates a collection
// whose "next" chain is a cycle of NON-empty pages.  Without the cap this test
// never returns.
func TestRangePages_NextCycle(t *testing.T) {

	pages := 0
	for range RangePages(cyclicCollection()) {
		pages++
	}

	assert.Equal(t, defaultMaxPages, pages)
}

// TestRangePages_WithMaxPages confirms WithMaxPages overrides the default page cap.
func TestRangePages_WithMaxPages(t *testing.T) {

	pages := 0
	for range RangePages(cyclicCollection(), WithMaxPages(7)) {
		pages++
	}

	assert.Equal(t, 7, pages)
}

// TestRangePages_WithMaxPages_LessThanOne pins the documented edge: a cap below
// one yields no pages at all.
func TestRangePages_WithMaxPages_LessThanOne(t *testing.T) {

	pages := 0
	for range RangePages(cyclicCollection(), WithMaxPages(0)) {
		pages++
	}

	assert.Equal(t, 0, pages)
}

// TestRangeDocuments_WithMaxPages confirms RangeDocuments forwards traversal
// options through to RangePages (one item per page in the cyclic fixture).
func TestRangeDocuments_WithMaxPages(t *testing.T) {

	documents := 0
	for range RangeDocuments(cyclicCollection(), WithMaxPages(5)) {
		documents++
	}

	assert.Equal(t, 5, documents)
}

// TestRangeDocuments_WithMaxDocuments confirms the document cap truncates
// mid-page: a single page of three items yields only two.
func TestRangeDocuments_WithMaxDocuments(t *testing.T) {

	collection := inlineCollection(
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	)

	got := collectIDs(RangeDocuments(collection, WithMaxDocuments(2)))
	assert.Equal(t, []string{"https://example.com/1", "https://example.com/2"}, got)
}

// TestRangeDocuments_WithMaxDocuments_LessThanOne pins the documented edge: a cap
// below one yields no documents at all.
func TestRangeDocuments_WithMaxDocuments_LessThanOne(t *testing.T) {
	assert.Empty(t, collectIDs(RangeDocuments(inlineCollection("https://example.com/1"), WithMaxDocuments(0))))
}

// TestRangeDocuments_DefaultMaxDocuments confirms the default document cap binds
// when the page cap is raised out of the way (one item per page in the cyclic
// fixture, so documents and pages count together).
func TestRangeDocuments_DefaultMaxDocuments(t *testing.T) {

	documents := 0
	for range RangeDocuments(cyclicCollection(), WithMaxPages(defaultMaxDocuments*2)) {
		documents++
	}

	assert.Equal(t, defaultMaxDocuments, documents)
}

// inlineCollection builds a single-page OrderedCollection with the given item IDs
// inline (no remote paging required).
func inlineCollection(itemIDs ...string) streams.Document {
	items := make([]any, 0, len(itemIDs))
	for _, id := range itemIDs {
		items = append(items, map[string]any{vocab.PropertyID: id})
	}
	return streams.NewDocument(map[string]any{
		vocab.PropertyType:  vocab.CoreTypeOrderedCollection,
		vocab.PropertyItems: items,
	})
}

// cyclicCollection builds a collection whose "next" chain is a cycle of two
// NON-empty pages -- the shape the empty-page check cannot catch.
func cyclicCollection() streams.Document {

	const pageA = "https://example.com/replies?page=a"
	const pageB = "https://example.com/replies?page=b"

	// Two non-empty pages that point at each other forever
	client := mapClient{documents: map[string]map[string]any{
		pageA: {
			vocab.PropertyID:    pageA,
			vocab.PropertyType:  vocab.CoreTypeCollectionPage,
			vocab.PropertyNext:  pageB,
			vocab.PropertyItems: []any{map[string]any{vocab.PropertyID: "https://example.com/reply/1"}},
		},
		pageB: {
			vocab.PropertyID:    pageB,
			vocab.PropertyType:  vocab.CoreTypeCollectionPage,
			vocab.PropertyNext:  pageA,
			vocab.PropertyItems: []any{map[string]any{vocab.PropertyID: "https://example.com/reply/2"}},
		},
	}}

	return streams.NewDocument(map[string]any{
		vocab.PropertyID:    "https://example.com/replies",
		vocab.PropertyType:  vocab.CoreTypeCollection,
		vocab.PropertyFirst: pageA,
	}, streams.WithClient(client))
}
