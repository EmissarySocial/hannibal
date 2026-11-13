package delivery

import (
	"iter"

	"github.com/benpate/remote"
)

// PipelineOption changes one setting of a Pipeline
type PipelineOption func(*Pipeline)

// WithBlockedHosts sets the function that returns the hosts that nothing should be queued for.
// It is called once per fan-out, so a circuit breaker can answer from a single query.
func WithBlockedHosts(blockedHosts func() (HostList, error)) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.blockedHosts = blockedHosts
	}
}

// WithRemoteOptions sets the function that returns the options added to each delivery's request,
// such as a circuit breaker's. It is called once per delivery, so an option may keep state.
func WithRemoteOptions(remoteOptions func() []remote.Option) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.remoteOptions = remoteOptions
	}
}

// WithCollections sets the application's own collections, which the fan-out expands into their members
func WithCollections(collections Collections) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.collections = collections
	}
}

// WithRelays sets the function that returns the relays for a public activity. It is called only
// for an activity whose `to` names Public, and it may return none.
func WithRelays(relays func(activity map[string]any) iter.Seq[string]) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.relays = relays
	}
}

// WithFilter sets the function that builds one fan-out's recipient test. It is called once per
// fan-out, so that it can load what it needs (block lists, for example) a single time.
func WithFilter(filter func(actorID string) (func(recipientURL string) bool, error)) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.filter = filter
	}
}

// WithFilterOne sets the test for a single recipient that is discovered inside a send task: the
// owner of a remote collection. It applies the same rules as WithFilter, by a lookup of its own.
func WithFilterOne(filterOne func(actorID string, recipientURL string) bool) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.filterOne = filterOne
	}
}

// WithEmail sets the function that delivers an activity to one mailto: recipient. Without it,
// such recipients are dropped.
func WithEmail(email func(activityID string, address string, activity map[string]any) error) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.email = email
	}
}

// WithProof sets the function that attaches an integrity proof to the copy of the activity
// that is sent. It receives the activity after its blind recipients are removed.
func WithProof(proof func(actorID string, activity map[string]any) (map[string]any, error)) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.proof = proof
	}
}

// WithAllowPrivateIPs lets deliveries connect to private and loopback addresses, for
// development between machines on a private network
func WithAllowPrivateIPs(allowPrivateIPs bool) PipelineOption {
	return func(pipeline *Pipeline) {
		pipeline.allowPrivateIPs = allowPrivateIPs
	}
}
