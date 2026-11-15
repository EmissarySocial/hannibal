package collections

import (
	"iter"
	"time"

	"github.com/benpate/hannibal/streams"
)

// RangeDocumentsBefore returns an iterator over Documents in the collection published before the given cursor.
func RangeDocumentsBefore(iterator iter.Seq[streams.Document], limit int64) iter.Seq[streams.Document] {

	return func(yield func(streams.Document) bool) {

		// Determine if the limit is non-zero.
		hasLimit := (limit > 0)

		// Convert to a time.Time object for comparison
		limitTime := time.Unix(limit, 0)

		// Scan through the range of documents
		for document := range iterator {

			// Filter out documents published after the "before" date. One published in that
			// same second is kept.
			if hasLimit && document.Published().After(limitTime) {
				continue
			}

			if !yield(document) {
				return
			}
		}
	}
}
