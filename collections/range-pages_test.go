//go:build localonly

package collections

import (
	"testing"

	"github.com/benpate/hannibal/streams"
)

// TestPages logs the IDs of up to 16 pages from a live Mastodon outbox.
func TestPages(t *testing.T) {

	doc := streams.NewDocument("https://mastodon.social/@benpate")
	outbox := doc.Outbox()

	pages := RangePages(outbox)

	index := 1
	for page := range pages {
		t.Log(page.ID())
		index++

		if index > 16 {
			break // okay, we get it.. you can load lots of pages.
		}
	}
}
