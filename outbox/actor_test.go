package outbox

import (
	"crypto/rand"
	"crypto/rsa"
	"slices"
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewActor confirms NewActor sets the ID, derives the default public key ID,
// and starts with an empty followers iterator.
func TestNewActor(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	actor := NewActor("https://example.com/users/alice", privateKey)

	assert.Equal(t, "https://example.com/users/alice", actor.ActorID())
	assert.Equal(t, "https://example.com/users/alice#main-key", actor.publicKeyID,
		"the default public key ID is the actor ID plus #main-key")
	require.NotNil(t, actor.followers)
	assert.Empty(t, slices.Collect(actor.followers))
}

// TestActorOptions confirms each ActorOption sets the matching field.
func TestActorOptions(t *testing.T) {

	client := streams.NewDefaultClient()
	followers := makeIterator("https://example.com/users/bob")

	actor := NewActor("https://example.com/users/alice", nil,
		WithPublicKey("https://example.com/users/alice#custom-key"),
		WithClient(client),
		WithFollowers(followers),
		WithAllowPrivateIPs(true),
	)

	assert.Equal(t, "https://example.com/users/alice#custom-key", actor.publicKeyID)
	assert.NotNil(t, actor.client)
	assert.Equal(t, []string{"https://example.com/users/bob"}, slices.Collect(actor.followers))
	assert.True(t, actor.allowPrivateIPs)
}

// TestActor_getClient confirms getClient returns the configured client, or a
// default when none was set.
func TestActor_getClient(t *testing.T) {

	// No client configured -> a default is created.
	bare := NewActor("https://example.com/users/alice", nil)
	assert.NotNil(t, bare.getClient())

	// A configured client is returned as-is.
	client := mockClient{inboxURL: "https://example.com/inbox"}
	configured := NewActor("https://example.com/users/alice", nil, WithClient(client))
	assert.Equal(t, client, configured.getClient())
}

// TestMakeIterator confirms makeIterator yields its values in order and honors an
// early stop.
func TestMakeIterator(t *testing.T) {

	iterator := makeIterator("a", "b", "c")

	var collected []string
	for value := range iterator {
		collected = append(collected, value)
	}
	assert.Equal(t, []string{"a", "b", "c"}, collected)

	// Early stop.
	var first []string
	for value := range makeIterator("x", "y", "z") {
		first = append(first, value)
		break
	}
	assert.Equal(t, []string{"x"}, first)
}
