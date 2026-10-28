package sender

import (
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/ranges"
	"github.com/stretchr/testify/require"
)

// TestSender_GetRecipients confirms getRecipients resolves an addressed actor to
// its single inbox through the Locator.
func TestSender_GetRecipients(t *testing.T) {

	activity := mapof.Any{vocab.PropertyTo: "https://test.actor.social/"}

	locator := testLocator{}
	rangeRecipients, err := getRecipients(locator, activity)

	require.Nil(t, err)

	recipients := ranges.Slice(rangeRecipients)
	require.Equal(t, 1, len(recipients))
	require.Equal(t, "https://testactor.social/inbox", recipients[0])
}

// TestLocator_GetFollowers confirms getRecipients expands an addressed followers
// collection into every follower's inbox, in order.
func TestLocator_GetFollowers(t *testing.T) {

	activity := mapof.Any{vocab.PropertyTo: "https://test.actor.social/followers"}

	locator := testLocator{}
	rangeRecipients, err := getRecipients(locator, activity)

	require.Nil(t, err)

	recipients := ranges.Slice(rangeRecipients)
	require.Equal(t, 3, len(recipients))
	require.Equal(t, "https://follower1.social/inbox", recipients[0])
	require.Equal(t, "https://follower2.social/inbox", recipients[1])
	require.Equal(t, "https://follower3.social/inbox", recipients[2])
}
