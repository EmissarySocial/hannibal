//go:build localonly

package collections

import (
	"testing"

	"github.com/benpate/hannibal/streams"
)

// TestDocuments logs the published dates of up to 100 documents from a live
// Mastodon outbox.
func TestDocuments(t *testing.T) {

	doc := streams.NewDocument("https://mastodon.social/@benpate")
	outbox := doc.Outbox()

	items := RangeDocuments(outbox)

	index := 1
	for item := range items {
		t.Log(item.Published())
		index++

		if index > 100 {
			break // okay, we get it.. you can load lots of documents...
		}
	}
}
