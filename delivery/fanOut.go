package delivery

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"iter"
	"slices"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/turbine/queue"
	"github.com/rs/zerolog/log"
)

// FanOut turns one activity into a send task for each of its recipients. Its arguments hold
// the activity under "activity", with its blind recipients still in place.
func (pipeline Pipeline) FanOut(args mapof.Any) queue.Result {

	const location = "hannibal.delivery.Pipeline.FanOut"

	activity := args.GetMap("activity")

	// RULE: The activity is sent as the actor that it names, so that actor must be one of ours
	actorID := firstReferenceID(activity[vocab.PropertyActor])

	if _, err := pipeline.signers(actorID); err != nil {
		return queue.Failure(derp.Wrap(err, location, "Unable to locate actor", "actorID: "+actorID))
	}

	// Decide who receives the activity, before anything is queued
	recipients, err := pipeline.recipients(actorID, activity)

	if err != nil {
		return queue.Error(derp.Wrap(err, location, "Listing recipients", "actorID: "+actorID))
	}

	// Build the one copy that every recipient is sent
	message, err := pipeline.message(actorID, activity)

	if err != nil {
		return queue.Failure(derp.Wrap(err, location, "Preparing activity", "actorID: "+actorID))
	}

	// Queue one task per recipient. A retry of this fan-out queues no second task for a
	// recipient whose first is still waiting, because each task has a signature.
	for _, recipientURL := range recipients {
		if err := pipeline.publish(actorID, recipientURL, message); err != nil {
			return queue.Error(derp.Wrap(err, location, "Queueing delivery", "recipient: "+recipientURL))
		}
	}

	// Signed, sealed, not yet delivered
	return queue.Success()
}

// recipients runs the stages that turn an activity's addressing into the list of people to
// deliver to: actor URLs, and mailto: URLs for those who receive email
func (pipeline Pipeline) recipients(actorID string, activity map[string]any) ([]string, error) {

	const location = "hannibal.delivery.Pipeline.recipients"

	// Load what the stages check against, once for the whole fan-out
	blocked, err := pipeline.loadBlockedHosts()

	if err != nil {
		return nil, derp.Wrap(err, location, "Loading blocked hosts")
	}

	allowed, err := pipeline.recipientFilter(actorID)

	if err != nil {
		return nil, derp.Wrap(err, location, "Loading recipient filter")
	}

	// Stack the stages. Nothing runs until the list is collected below.
	var expandError error

	extracted := Extract(activity)
	withRelays := ExpandRelays(extracted, pipeline.relaysFor(activity))
	expanded := ExpandCollections(withRelays, pipeline.collections, func(err error) {
		expandError = err
	})
	unique := Deduplicate(expanded)
	reachable := SkipBlockedHosts(unique, blocked)
	result := slices.Collect(Filter(reachable, allowed))

	// RULE: A collection that could not be read fails the whole fan-out, before anything is
	// queued, so that the retry delivers to everyone and not to a partial list
	if expandError != nil {
		return nil, derp.Wrap(expandError, location, "Expanding collection")
	}

	return result, nil
}

// blockedHosts returns the hosts that nothing should be queued for, read once
func (pipeline Pipeline) loadBlockedHosts() (HostList, error) {

	if pipeline.blockedHosts == nil {
		return noHosts{}, nil
	}

	return pipeline.blockedHosts()
}

// noHosts is the HostList of an application that blocks no hosts
type noHosts struct{}

// Contains returns FALSE for every host
func (noHosts) Contains(string) bool {
	return false
}

// recipientFilter returns the test that every recipient of this fan-out must pass
func (pipeline Pipeline) recipientFilter(actorID string) (func(string) bool, error) {

	if pipeline.filter == nil {
		return allowAll, nil
	}

	return pipeline.filter(actorID)
}

// allowAll is the recipient test of an application that excludes no one
func allowAll(string) bool {
	return true
}

// relaysFor returns the relays for an activity. The stage that consumes it reads it only when
// the activity is public.
func (pipeline Pipeline) relaysFor(activity map[string]any) iter.Seq[string] {

	return func(yield func(string) bool) {

		if pipeline.relays == nil {
			return
		}

		for relayURL := range pipeline.relays(activity) {
			if !yield(relayURL) {
				return
			}
		}
	}
}

// outboundMessage is the one copy of an activity that every recipient of a fan-out is sent
type outboundMessage struct {
	activityID    string         // The activity's id, or a hash of its body when it has none
	activity      map[string]any // The activity without blind recipients, for recipients who receive email
	body          string         // The activity as it is sent: stripped, proven, and serialized once
	digest        string         // Value of the `Digest` header for this body
	contentDigest string         // Value of the `Content-Digest` header for this body
}

// message removes the blind recipients, attaches the integrity proof, and serializes the result once
func (pipeline Pipeline) message(actorID string, activity map[string]any) (outboundMessage, error) {

	const location = "hannibal.delivery.Pipeline.message"

	// RULE: Strip first. A proof made over the blind recipients would fail once they are removed.
	stripped := StripBlindRecipients(activity)
	proven := pipeline.prove(actorID, stripped)

	serialized, err := json.Marshal(proven)

	if err != nil {
		return outboundMessage{}, derp.Wrap(err, location, "Serializing activity")
	}

	hash := sha256.Sum256(serialized)
	encoded := base64.StdEncoding.EncodeToString(hash[:])

	// An activity with no id of its own is identified by its content
	activityID := firstReferenceID(activity[vocab.PropertyID])

	if activityID == "" {
		activityID = "sha256:" + hex.EncodeToString(hash[:])
	}

	return outboundMessage{
		activityID:    activityID,
		activity:      stripped,
		body:          string(serialized),
		digest:        "SHA-256=" + encoded,
		contentDigest: "sha-256=:" + encoded + ":",
	}, nil
}

// prove attaches the integrity proof. A proof that cannot be made is logged, and the activity
// is sent without one: recipients that do not check proofs lose nothing.
func (pipeline Pipeline) prove(actorID string, stripped map[string]any) map[string]any {

	if pipeline.proof == nil {
		return stripped
	}

	proven, err := pipeline.proof(actorID, stripped)

	if err != nil {
		derp.Report(derp.Wrap(err, "hannibal.delivery.Pipeline.prove", "Unable to attach integrity proof. Sending without one.", "actorID: "+actorID))
		return stripped
	}

	return proven
}

// publish queues the delivery of one message to one recipient, by ActivityPub or by email
func (pipeline Pipeline) publish(actorID string, recipientURL string, message outboundMessage) error {

	const location = "hannibal.delivery.Pipeline.publish"

	if IsEmail(recipientURL) {
		return pipeline.publishEmail(recipientURL, message)
	}

	// RULE: Only a web address can have an inbox
	if HostOf(recipientURL) == "" {
		log.Warn().Str("recipient", recipientURL).Msg("hannibal.delivery: dropped a recipient that is not a URL")
		return nil
	}

	task := queue.NewTask(
		TaskSend,
		mapof.Any{
			"actor":         actorID,
			"recipient":     recipientURL,
			"activityId":    message.activityID,
			"body":          message.body,
			"digest":        message.digest,
			"contentDigest": message.contentDigest,
		},
		queue.WithSignature(message.activityID+" "+recipientURL),
		queue.WithRetryMax(sendRetryMax),
		queue.WithBackoffMax(sendBackoffMax),
	)

	if err := pipeline.queue.Publish(task); err != nil {
		return derp.Wrap(err, location, "Publishing send task")
	}

	return nil
}

// publishEmail hands one mailto: recipient to the application, which sends email its own way
func (pipeline Pipeline) publishEmail(address string, message outboundMessage) error {

	if pipeline.email == nil {
		log.Warn().Msg("hannibal.delivery: dropped an email recipient, because no email handler is set")
		return nil
	}

	if err := pipeline.email(message.activityID, address, message.activity); err != nil {
		return derp.Wrap(err, "hannibal.delivery.Pipeline.publishEmail", "Delivering by email")
	}

	return nil
}

// firstReferenceID returns the id of the first reference in a property value, or an empty string
func firstReferenceID(value any) string {

	ids := referenceIDs(value)

	if len(ids) == 0 {
		return ""
	}

	return ids[0]
}
