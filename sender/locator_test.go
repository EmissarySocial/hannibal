package sender

import (
	"iter"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/ranges"
)

// testLocator is a stub Locator that resolves testActor and fixed inbox lists.
type testLocator struct{}

// Actor returns testActor when the address is its ID, and a NotFound error otherwise.
func (t testLocator) Actor(address string) (Actor, error) {

	result := testActor{} // nolint:scopeguard - it's just a test, bro.

	if address == result.ActorID() {
		return result, nil
	}

	return nil, derp.NotFound("testLocator.Actor", "Unable to load actor")
}

// Recipient returns one inbox for the test actor's URL, three for its followers
// collection, and an empty iterator for any other address.
func (t testLocator) Recipient(address string) (iter.Seq[string], error) {

	switch address {
	case "https://test.actor.social/":
		return ranges.Values("https://testactor.social/inbox"), nil

	case "https://test.actor.social/followers":
		return ranges.Values(
			"https://follower1.social/inbox",
			"https://follower2.social/inbox",
			"https://follower3.social/inbox",
		), nil
	}

	return ranges.Empty[string](), nil
}
