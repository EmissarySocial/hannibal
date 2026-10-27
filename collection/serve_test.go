package collection

import (
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/******************************************
 * Test Helpers
 ******************************************/

// newContext builds an Echo context plus its response recorder for a GET request
// to the given target (which may include a query string).
func newContext(target string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	return e.NewContext(request, recorder), recorder
}

// newContextWithAccept is like newContext but sets the Accept header, used to
// exercise content-type negotiation in serveJSON.
func newContextWithAccept(target string, accept string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("Accept", accept)
	recorder := httptest.NewRecorder()
	return e.NewContext(request, recorder), recorder
}

// counter returns a CounterFunc that always reports the given total.
func counter(total int64) CounterFunc {
	return func() (int64, error) { return total, nil }
}

// makeActivities builds n activity maps. If fullObjects is true, each carries an
// extra field so it is rendered as a full object; otherwise each is id-only.
func makeActivities(n int, fullObjects bool) []mapof.Any {
	result := make([]mapof.Any, 0, n)
	for i := 0; i < n; i++ {
		activity := mapof.Any{vocab.PropertyID: "https://example.com/activity/" + itoa(i)}
		if fullObjects {
			activity[vocab.PropertyType] = vocab.ActivityTypeCreate
		}
		result = append(result, activity)
	}
	return result
}

// iterator returns an IteratorFunc that yields the given activities, ignoring the
// startIndex (the storage layer is assumed to pre-filter).
func iterator(activities []mapof.Any) IteratorFunc {
	return func(_ string) (iter.Seq[mapof.Any], error) {
		return func(yield func(mapof.Any) bool) {
			for _, activity := range activities {
				if !yield(activity) {
					return
				}
			}
		}, nil
	}
}

// decodeBody parses the recorded JSON response body into a mapof.Any.
func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) mapof.Any {
	t.Helper()
	result := mapof.NewAny()
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
	return result
}

// itoa is a tiny int-to-string helper to avoid importing strconv in fixtures.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}

/******************************************
 * serveOrderedCollection branches
 ******************************************/

// TestServe_Empty confirms an empty collection returns an OrderedCollection with
// totalItems 0 and no items / first page.
func TestServe_Empty(t *testing.T) {

	ctx, recorder := newContext("https://example.com/outbox")

	err := Serve(ctx, "https://example.com/outbox", counter(0), iterator(nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)

	body := decodeBody(t, recorder)
	assert.Equal(t, vocab.CoreTypeOrderedCollection, body[vocab.PropertyType])
	assert.EqualValues(t, 0, body[vocab.PropertyTotalItems])
	assert.NotContains(t, body, vocab.PropertyOrderedItems)
	assert.NotContains(t, body, vocab.PropertyFirst)
}

// TestServe_SinglePage confirms a collection smaller than one page returns the
// items inline in orderedItems (no paging).
func TestServe_SinglePage(t *testing.T) {

	activities := makeActivities(3, false)
	ctx, recorder := newContext("https://example.com/outbox")

	err := Serve(ctx, "https://example.com/outbox", counter(3), iterator(activities))
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	assert.Equal(t, vocab.CoreTypeOrderedCollection, body[vocab.PropertyType])
	assert.EqualValues(t, 3, body[vocab.PropertyTotalItems])

	items, ok := body[vocab.PropertyOrderedItems].([]any)
	require.True(t, ok, "orderedItems should be present and a slice")
	assert.Len(t, items, 3)

	// id-only activities render as plain string IDs.
	assert.Equal(t, "https://example.com/activity/0", items[0])

	// A single-page collection does not advertise a first page.
	assert.NotContains(t, body, vocab.PropertyFirst)
}

// TestServe_MultiPage confirms a collection at or above the page size returns a
// "first" page link instead of inline items.
func TestServe_MultiPage(t *testing.T) {

	ctx, recorder := newContext("https://example.com/outbox")

	// totalItems >= maxItemsPerPage triggers paging.
	err := Serve(ctx, "https://example.com/outbox", counter(maxItemsPerPage), iterator(nil))
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	assert.Equal(t, vocab.CoreTypeOrderedCollection, body[vocab.PropertyType])
	assert.Equal(t, "https://example.com/outbox?after=FIRST", body[vocab.PropertyFirst])
	assert.NotContains(t, body, vocab.PropertyOrderedItems)
}

// TestServe_CountError confirms a failure in the count function is surfaced.
func TestServe_CountError(t *testing.T) {

	ctx, _ := newContext("https://example.com/outbox")

	failingCounter := func() (int64, error) {
		return 0, derp.Internal("test", "count failed")
	}

	err := Serve(ctx, "https://example.com/outbox", failingCounter, iterator(nil))
	require.Error(t, err)
}

// TestServe_SinglePage_IteratorError confirms an iterator failure on the inline
// path is surfaced.
func TestServe_SinglePage_IteratorError(t *testing.T) {

	ctx, _ := newContext("https://example.com/outbox")

	failingIterator := func(_ string) (iter.Seq[mapof.Any], error) {
		return nil, derp.Internal("test", "iterator failed")
	}

	err := Serve(ctx, "https://example.com/outbox", counter(3), failingIterator)
	require.Error(t, err)
}

// TestServe_AppliesConfig confirms config options are written into the
// OrderedCollection response.
func TestServe_AppliesConfig(t *testing.T) {

	ctx, recorder := newContext("https://example.com/outbox")

	err := Serve(ctx, "https://example.com/outbox", counter(0), iterator(nil),
		WithAttributedTo("https://example.com/actor"),
		WithAudience("https://example.com/followers"),
		WithSSEEndpoint("https://example.com/sse"),
	)
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	assert.Equal(t, "https://example.com/actor", body[vocab.PropertyAttributedTo])
	assert.Equal(t, "https://example.com/followers", body[vocab.PropertyAudience])
	assert.Equal(t, "https://example.com/sse", body[vocab.PropertyEventStream])
}

/******************************************
 * serveOrderedCollectionPage branches
 ******************************************/

// TestServe_Page_First confirms the "after=FIRST" magic value returns the first
// page of items as an OrderedCollectionPage.
func TestServe_Page_First(t *testing.T) {

	activities := makeActivities(3, false)
	ctx, recorder := newContext("https://example.com/outbox?after=FIRST")

	err := Serve(ctx, "https://example.com/outbox", counter(100), iterator(activities))
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	assert.Equal(t, vocab.CoreTypeOrderedCollectionPage, body[vocab.PropertyType])
	assert.Equal(t, "https://example.com/outbox", body[vocab.PropertyPartOf])
	assert.Equal(t, "https://example.com/outbox?after=FIRST", body[vocab.PropertyID])

	items, ok := body[vocab.PropertyOrderedItems].([]any)
	require.True(t, ok)
	assert.Len(t, items, 3)

	// Fewer than a full page -> no "next" link.
	assert.NotContains(t, body, vocab.PropertyNext)
}

// TestServe_Page_Next confirms a full page advertises a "next" link pointing at
// the last item's ID.
func TestServe_Page_Next(t *testing.T) {

	activities := makeActivities(maxItemsPerPage, false)
	ctx, recorder := newContext("https://example.com/outbox?after=SOMEID")

	err := Serve(ctx, "https://example.com/outbox", counter(1000), iterator(activities))
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	assert.Equal(t, vocab.CoreTypeOrderedCollectionPage, body[vocab.PropertyType])

	items, ok := body[vocab.PropertyOrderedItems].([]any)
	require.True(t, ok)
	assert.Len(t, items, maxItemsPerPage)

	// A full page advertises the next page, keyed on the last item's ID.
	lastID := "https://example.com/activity/" + itoa(maxItemsPerPage-1)
	assert.Equal(t, "https://example.com/outbox?after="+lastID, body[vocab.PropertyNext])
}

// TestServe_Page_LimitsToPageSize confirms getItems never returns more than the
// page size, even when the iterator yields more.
func TestServe_Page_LimitsToPageSize(t *testing.T) {

	// The iterator offers far more than one page.
	activities := makeActivities(maxItemsPerPage*2, false)
	ctx, recorder := newContext("https://example.com/outbox?after=FIRST")

	err := Serve(ctx, "https://example.com/outbox", counter(1000), iterator(activities))
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	items, ok := body[vocab.PropertyOrderedItems].([]any)
	require.True(t, ok)
	assert.Len(t, items, maxItemsPerPage, "a page must be capped at maxItemsPerPage")
}

// TestServe_Page_IteratorError confirms an iterator failure on the page path is
// surfaced.
func TestServe_Page_IteratorError(t *testing.T) {

	ctx, _ := newContext("https://example.com/outbox?after=FIRST")

	failingIterator := func(_ string) (iter.Seq[mapof.Any], error) {
		return nil, derp.Internal("test", "iterator failed")
	}

	err := Serve(ctx, "https://example.com/outbox", counter(100), failingIterator)
	require.Error(t, err)
}

/******************************************
 * activityValue
 ******************************************/

// TestActivityValue confirms an id-only activity collapses to its ID string,
// while a richer activity is returned whole.
func TestActivityValue(t *testing.T) {

	// id-only -> bare string.
	idOnly := mapof.Any{vocab.PropertyID: "https://example.com/1"}
	assert.Equal(t, "https://example.com/1", activityValue(idOnly))

	// id plus other fields -> the full map.
	full := mapof.Any{
		vocab.PropertyID:   "https://example.com/1",
		vocab.PropertyType: vocab.ActivityTypeCreate,
	}
	assert.Equal(t, full, activityValue(full))

	// No id at all -> the full map (cannot collapse).
	noID := mapof.Any{vocab.PropertyType: vocab.ActivityTypeCreate}
	assert.Equal(t, noID, activityValue(noID))
}

// TestServe_ContentTypeNegotiation confirms serveJSON returns compact JSON for
// an ActivityPub Accept header and pretty-printed JSON otherwise. Both must
// decode to the same data.
func TestServe_ContentTypeNegotiation(t *testing.T) {

	activities := makeActivities(2, false)

	t.Run("ActivityPub Accept -> compact", func(t *testing.T) {
		ctx, recorder := newContextWithAccept("https://example.com/outbox", vocab.ContentTypeActivityPub)

		err := Serve(ctx, "https://example.com/outbox", counter(2), iterator(activities))
		require.NoError(t, err)

		body := decodeBody(t, recorder)
		assert.EqualValues(t, 2, body[vocab.PropertyTotalItems])
		// Compact output has no indentation.
		assert.NotContains(t, recorder.Body.String(), "\n    ")
	})

	t.Run("browser Accept -> pretty", func(t *testing.T) {
		ctx, recorder := newContextWithAccept("https://example.com/outbox", "text/html")

		err := Serve(ctx, "https://example.com/outbox", counter(2), iterator(activities))
		require.NoError(t, err)

		body := decodeBody(t, recorder)
		assert.EqualValues(t, 2, body[vocab.PropertyTotalItems])
		// Pretty output is indented with four spaces.
		assert.Contains(t, recorder.Body.String(), "\n    ")
	})
}

// TestServe_FullObjectsRenderWhole confirms multi-field activities are rendered
// as objects (not collapsed to IDs) in the inline page.
func TestServe_FullObjectsRenderWhole(t *testing.T) {

	activities := makeActivities(2, true) // each has id + type
	ctx, recorder := newContext("https://example.com/outbox")

	err := Serve(ctx, "https://example.com/outbox", counter(2), iterator(activities))
	require.NoError(t, err)

	body := decodeBody(t, recorder)
	items, ok := body[vocab.PropertyOrderedItems].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)

	// Each item is a JSON object, not a bare string.
	first, ok := items[0].(map[string]any)
	require.True(t, ok, "a multi-field activity should render as an object")
	assert.Equal(t, vocab.ActivityTypeCreate, first[vocab.PropertyType])
}
