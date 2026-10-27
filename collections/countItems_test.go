//go:build localonly

package collections

import (
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/stretchr/testify/require"
)

// TestCountItems logs the item counts of live remote collections, failing only
// when a load or a count returns an error.
func TestCountItems(t *testing.T) {

	do := func(url string) {
		client := streams.NewDefaultClient()
		collection, err := client.Load(url)
		require.Nil(t, err)

		count, err := CountItems(collection)
		require.Nil(t, err)
		t.Logf("%s: %d", url, count)
	}

	do("https://mastodon.social/users/benpate/outbox")
	// do("https://social.wizard.casa/users/benpate/outbox")
	// do("https://social.wizard.casa/users/benpate/followers")
	do("https://infosec.exchange/users/tinker/statuses/115276098511707960/likes")
}
