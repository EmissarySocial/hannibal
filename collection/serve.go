// Package collection is an HTTP interface for GET-ing activities from
// an ActivityPub collection.
package collection

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/ranges"
)

// maxItemsPerPage is the maximum number of items that will be returned in a single page of an OrderedCollection
const maxItemsPerPage = 60

// firstPage is the value of the `after` parameter that asks for the first page of a collection
const firstPage = "FIRST"

// Serve writes an ActivityStreams OrderedCollection, or the OrderedCollectionPage that the
// request's `after` parameter names. The iterator must already apply the caller's permissions.
func Serve(writer http.ResponseWriter, request *http.Request, collectionID string, countFunc CounterFunc, iteratorFunc IteratorFunc, options ...Option) error {

	config := NewConfig(options...)

	// If we have an "after" parameter, then return the OrderedCollectionPage
	// corresponding to all activities after the provided ID
	if after := request.URL.Query().Get("after"); after != "" {
		return serveOrderedCollectionPage(writer, request, collectionID, iteratorFunc, after)
	}

	// Otherwise, return the OrderedCollection container
	return serveOrderedCollection(writer, request, collectionID, countFunc, iteratorFunc, config)
}

// ServeEmpty generates an empty OrderedCollection and returns it via HTTP.
func ServeEmpty(writer http.ResponseWriter, request *http.Request, collectionID string) error {

	// An empty collection is just a summary whose count is zero.  This deliberately does NOT
	// marshal a streams.OrderedCollection: that struct tags TotalItems as `omitempty`, so the
	// zero this function exists to report is exactly the value that would be dropped.
	return ServeSummary(writer, request, collectionID, 0)
}

// ServeSummary generates an OrderedCollection that publishes its SIZE but not its MEMBERS, and
// returns it via HTTP: `id`, `type`, and `totalItems`, with no `first` link and no `orderedItems`.
func ServeSummary(writer http.ResponseWriter, request *http.Request, collectionID string, totalItems int64, options ...Option) error {

	// The absence of `first` is the signal: Mastodon reads a collection with a count and no
	// first page as one the actor has chosen to hide. The README explains when to use this.
	result := mapof.Any{
		vocab.AtContext:          vocab.ContextTypeActivityStreams,
		vocab.PropertyType:       vocab.CoreTypeOrderedCollection,
		vocab.PropertyID:         collectionID,
		vocab.PropertyTotalItems: totalItems,
	}

	NewConfig(options...).Apply(&result)

	return serveJSON(writer, request, result)
}

// serveOrderedCollection serves the collection itself: its count, and either its items or a
// link to its first page
func serveOrderedCollection(writer http.ResponseWriter, request *http.Request, collectionID string, countFunc CounterFunc, iteratorFunc IteratorFunc, config Config) error {

	const location = "collection.serveOrderedCollection"

	// Count the total number of activities in this outbox
	totalItems, err := countFunc()

	if err != nil {
		return derp.Wrap(err, location, "Unable to count activities in outbox storage")
	}

	result := mapof.Any{
		vocab.AtContext:          vocab.ContextTypeActivityStreams,
		vocab.PropertyType:       vocab.CoreTypeOrderedCollection,
		vocab.PropertyID:         collectionID,
		vocab.PropertyTotalItems: totalItems,
	}

	config.Apply(&result)

	// If there are no items, then return an empty collection
	if totalItems == 0 {
		return serveJSON(writer, request, result)
	}

	// If we only have one page worth of items, then return it directly in the "orderedItems" property
	if totalItems < maxItemsPerPage {
		orderedItems, _, err := getItems(iteratorFunc, "")

		if err != nil {
			return derp.Wrap(err, location, "Unable to retrieve outbox activities")
		}

		result[vocab.PropertyOrderedItems] = orderedItems
		return serveJSON(writer, request, result)

	}

	// Otherwise, break the collection into pages. Return the FIRST page URL
	result[vocab.PropertyFirst] = pageURL(collectionID, firstPage)
	return serveJSON(writer, request, result)
}

// serveOrderedCollectionPage serves a single page of activities from the OrderedCollection
// starting after the provided activity ID.
func serveOrderedCollectionPage(writer http.ResponseWriter, request *http.Request, collectionID string, iteratorFunc IteratorFunc, after string) error {

	const location = "collection.serveOrderedCollectionPage"

	// Remove the magic "FIRST" value (if present) to send to the getItems() function
	afterParam := iif(after == firstPage, "", after)

	// Get the activities in the collection
	orderedItems, lastID, err := getItems(iteratorFunc, afterParam)

	if err != nil {
		return derp.Wrap(err, location, "Unable to retrieve outbox activities")
	}

	// Build the OrderedCollectionPage response
	result := mapof.Any{
		vocab.AtContext:            vocab.ContextTypeActivityStreams,
		vocab.PropertyType:         vocab.CoreTypeOrderedCollectionPage,
		vocab.PropertyID:           pageURL(collectionID, after),
		vocab.PropertyPartOf:       collectionID,
		vocab.PropertyOrderedItems: orderedItems,
	}

	// Set the "next" after URL (if applicable).
	// RULE: A page whose last item has no id cannot name the page after it. An empty cursor
	// would be read as a request for the collection itself, and paging would start over.
	if (len(orderedItems) >= maxItemsPerPage) && (lastID != "") {
		result[vocab.PropertyNext] = pageURL(collectionID, lastID)
	}

	// Done!
	return serveJSON(writer, request, result)
}

// pageURL returns the address of the page that follows the given item. The cursor is an
// item's id, which is a URL of its own, so it is escaped.
func pageURL(collectionID string, after string) string {
	return collectionID + "?after=" + url.QueryEscape(after)
}

// getItems retrieves all items from the collection after the provided "lastID".
func getItems(iteratorFunc IteratorFunc, after string) (items []any, lastID string, err error) {

	const location = "collection.getItems"

	// Initialize the result so that we can return an empty array if there are no items in the collection
	items = make([]any, 0)

	// Get an iterator over all (matching) activities in the Storage adapter
	activities, err := iteratorFunc(after)

	if err != nil {
		return nil, "", derp.Wrap(err, location, "Unable to retrieve outbox activities")
	}

	// Do not retrieve more than 60 items
	activities = ranges.Limit(maxItemsPerPage, activities)

	for activity := range activities {
		lastID = activity.GetString(vocab.PropertyID)
		items = append(items, activityValue(activity))
	}

	return items, lastID, nil
}

// activityValue returns either the activity ID as a string if only this value is present,
// or a full mapof.Any if more than just the ID is present
func activityValue(activity mapof.Any) any {

	if activityID := activity.GetString(vocab.PropertyID); (len(activity) == 1) && (activityID != "") {
		return activityID
	}

	return activity
}

// serveJSON writes an ActivityPub document: compact for another server, and indented for a browser
func serveJSON(writer http.ResponseWriter, request *http.Request, data any) error {

	const location = "collection.serveJSON"

	var body []byte
	var err error

	if hannibal.IsActivityPubRequest(request) {
		body, err = json.Marshal(data)
	} else {
		body, err = json.MarshalIndent(data, "", "    ")
	}

	if err != nil {
		return derp.Wrap(err, location, "Serializing collection")
	}

	// RULE: Every response is labelled as ActivityPub, the collection and its pages alike
	writer.Header().Set("Content-Type", vocab.ContentTypeActivityPub)
	writer.WriteHeader(http.StatusOK)

	if _, err := writer.Write(body); err != nil {
		return derp.Wrap(err, location, "Writing collection")
	}

	return nil
}
