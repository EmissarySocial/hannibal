package collections

import (
	"github.com/benpate/derp"
	"github.com/benpate/hannibal/streams"
)

// CountItems returns a collection's `totalItems`, or counts its items page by page when it
// names none. It accepts the options of RangePages, which bound how much is read.
func CountItems(collection streams.Document, options ...Option) (count int, err error) {

	const location = "hannibal.collections.CountItems"

	// If the collection does not exist, then there are no items.
	if collection.IsNil() {
		return 0, nil
	}

	// RULE: A panic is an error, and never a count. Reporting it as zero items would tell
	// the caller that the collection is empty.
	defer func() {
		if recovered := recover(); recovered != nil {
			count = 0
			err = derp.Internal(location, "Recovered from a panic while counting", recovered, collection.ID())
		}
	}()

	// If this is not already a "map" then load the complete document.
	collection = collection.LoadLink()

	// If the collection already reports the `TotalItems` count, then just use that.
	// This is the best case scenario.
	if totalItems := collection.TotalItems(); totalItems > 0 {
		return totalItems, nil
	}

	// Otherwise, we have to do your work for you.  Let's start paging and counting...
	var result int

	// Retrieve each page in the collection and count the number of items it contains.
	for page := range RangePages(collection, options...) {

		// Get the number of items in this page
		pageCount := page.Items().Len()

		// If this page is empty, then we don't need to load any more pages
		if pageCount == 0 {
			break
		}

		// Increment the total count
		result += pageCount
	}

	// Return the number of items found
	return result, nil
}
