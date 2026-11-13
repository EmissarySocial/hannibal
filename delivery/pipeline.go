package delivery

import (
	"crypto"
	"iter"

	"github.com/benpate/hannibal/streams"
	"github.com/benpate/remote"
	"github.com/benpate/turbine/queue"
)

// Signer is the actor that an activity is sent as, with the key that signs its requests
type Signer interface {

	// ActorID returns the URL of the actor
	ActorID() string

	// PrivateKey returns the id of the actor's public key and the private key that matches it
	PrivateKey() (publicKeyID string, privateKey crypto.PrivateKey)
}

// Pipeline delivers outbound activities. It holds the application's side of each decision: who
// signs, which collections are local, who is excluded, and where email goes.
type Pipeline struct {
	queue           *queue.Queue                                                           // Queue that the send tasks are published to
	client          streams.Client                                                         // Loads actor documents, through the application's cache
	signers         func(actorID string) (Signer, error)                                   // Returns the signer for a local actor
	blockedHosts    func() (HostList, error)                                               // Returns the hosts that nothing is queued for, once per fan-out
	remoteOptions   func() []remote.Option                                                 // Returns the options added to one delivery's request, built new each time
	collections     Collections                                                            // The application's own collections
	relays          func(activity map[string]any) iter.Seq[string]                         // Relays that receive a public activity
	filter          func(actorID string) (func(recipientURL string) bool, error)           // Builds the test that every recipient of one fan-out must pass
	filterOne       func(actorID string, recipientURL string) bool                         // Tests one recipient found inside a send task
	email           func(activityID string, address string, activity map[string]any) error // Delivers the activity to one mailto: recipient
	proof           func(actorID string, activity map[string]any) (map[string]any, error)  // Attaches an integrity proof to the copy that is sent
	allowPrivateIPs bool                                                                   // TRUE if deliveries may connect to private addresses
}

// New returns a Pipeline that publishes its send tasks to the queue, loads actors through the
// client, and signs as the actors that the signers function returns
func New(taskQueue *queue.Queue, client streams.Client, signers func(actorID string) (Signer, error), options ...PipelineOption) Pipeline {

	result := Pipeline{
		queue:       taskQueue,
		client:      client,
		signers:     signers,
		collections: noCollections{},
	}

	for _, option := range options {
		option(&result)
	}

	return result
}

// noCollections is the Collections of an application that has none of its own
type noCollections struct{}

// IsCollection returns FALSE for every URL
func (noCollections) IsCollection(string) bool {
	return false
}

// Members returns no members
func (noCollections) Members(string) (iter.Seq[string], error) {
	return func(func(string) bool) {}, nil
}
