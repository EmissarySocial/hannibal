package outbox

import (
	"crypto"
	"iter"

	"github.com/benpate/hannibal/streams"
)

// Actor represents an ActivityPub actor that can send ActivityPub messages
// https://www.w3.org/TR/activitypub/#actors
type Actor struct {

	// Required values passed to NewActor function
	actorID    string
	privateKey crypto.PrivateKey

	// Optional values set via With() options
	publicKeyID     string
	client          streams.Client
	followers       iter.Seq[string]
	allowPrivateIPs bool
	// A queue field may be reintroduced here if outbox delivery moves back onto a task queue.
}

/******************************************
 * Lifecycle Methods
 ******************************************/

// NewActor returns a fully initialized Actor object, and applies optional settings as provided
func NewActor(actorID string, privateKey crypto.PrivateKey, options ...ActorOption) Actor {

	// Set Default Values
	result := Actor{
		actorID:     actorID,
		publicKeyID: actorID + "#main-key",
		privateKey:  privateKey,
		followers:   func(_ func(string) bool) {}, // Default is an empty iterator
	}

	// Apply additional options
	result.With(options...)
	return result
}

// With applies one or more options to an Actor
func (actor *Actor) With(options ...ActorOption) {
	for _, option := range options {
		option(actor)
	}
}

// ActorID returns the URL that identifies this Actor.
func (actor *Actor) ActorID() string {
	return actor.actorID
}

/******************************************
 * Internal / Helper Methods
 ******************************************/

// getClient returns the hannibal Client to use when retrieving
// JSON-LD data.  If the Actor does not include a custom client,
// then a default HTTP-only client is used instead.
func (actor *Actor) getClient() streams.Client {

	if actor.client != nil {
		return actor.client
	}

	return streams.NewDefaultClient()
}
