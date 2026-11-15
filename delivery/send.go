package delivery

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/remote"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/turbine/queue"
	"github.com/benpate/uri"
	"github.com/rs/zerolog/log"
)

// maxOwnerCandidates bounds how many actors a send task loads while it looks for the owner of
// a remote collection, because the candidates come from documents on other servers
const maxOwnerCandidates = 16

// minimumRequeueDelay is the shortest wait before a rescheduled delivery runs again
const minimumRequeueDelay = time.Second

// maximumRequeueDelay is the longest wait that a host can ask of a rescheduled delivery
const maximumRequeueDelay = time.Hour

// maximumDeliveryAge is how long a host may keep rescheduling one delivery with a rate limit.
// It matches the span that the retries of a failing delivery cover.
const maximumDeliveryAge = 7 * 24 * time.Hour

// maxReplyParents bounds how many posts a send task loads to learn who wrote them
const maxReplyParents = 4

// argumentQueuedAt names the send task argument that holds when the delivery was first queued,
// in Unix seconds. It outlasts a reschedule, which the task's own counters do not.
const argumentQueuedAt = "queuedAt"

// Send delivers one activity to one recipient's own inbox. Its arguments are those that
// FanOut queued: "actor", "recipient", and the serialized "body".
func (pipeline Pipeline) Send(args mapof.Any) queue.Result {

	const location = "hannibal.delivery.Pipeline.Send"

	actorID := args.GetString("actor")
	recipientURL := args.GetString("recipient")
	body := args.GetString("body")

	// RULE: A task must carry something to deliver
	if body == "" {
		return queue.Failure(derp.Internal(location, "Task has no body to send", "recipient: "+recipientURL))
	}

	signer, err := pipeline.signers(actorID)

	if err != nil {
		return signerResult(err, location, actorID)
	}

	// Find the actor to deliver to. Anything else ends the task here.
	target, result, found := pipeline.resolve(actorID, recipientURL, body)

	if !found {
		return result
	}

	// RULE: Every delivery goes to the recipient's own inbox, never a shared one
	inboxURL := target.Inbox().ID()

	// RULE: The inbox must be on the actor's own origin. Otherwise a crowd of fake followers could
	// each name a victim's server as their inbox, and every post would flood it.
	if uri.NotSameOrigin(inboxURL, target.ID()) {
		err := derp.Forbidden(location, "Inbox is not on the actor's host. Delivery dropped.", "actor: "+target.ID(), "inbox: "+inboxURL)
		derp.Report(err)
		return queue.Failure(err)
	}

	// Deliver, then choose what happens to the task
	return pipeline.result(HostOf(inboxURL), args.GetInt64(argumentQueuedAt), pipeline.post(signer, inboxURL, body))
}

// signerResult returns what happens to a task whose signing actor could not be loaded
func signerResult(err error, location string, actorID string) queue.Result {

	wrapped := derp.Wrap(err, location, "Unable to load the signing actor", "actorID: "+actorID)

	// RULE: An actor that does not exist will not come back, so the task ends
	if derp.IsNotFoundOrGone(err) {
		return queue.Failure(wrapped)
	}

	// Any other error, such as a database that did not answer, may pass
	return queue.Error(wrapped)
}

// resolve finds the actor that a recipient URL stands for: the actor itself, or the owner of a
// remote collection. When it finds none, it returns the Result that ends the task.
func (pipeline Pipeline) resolve(actorID string, recipientURL string, body string) (streams.Document, queue.Result, bool) {

	const location = "hannibal.delivery.Pipeline.resolve"

	// Load the recipient through the application's cache
	document, loadError := pipeline.client.Load(recipientURL)

	if (loadError == nil) && document.IsActor() {
		return document, queue.Result{}, true
	}

	// Anything else may be a remote collection, which is not expanded: it stands for its owner
	if owner, found := pipeline.collectionOwner(recipientURL, body); found {
		return pipeline.resolveOwner(actorID, owner)
	}

	// RULE: A recipient that is gone will not come back, so it is not retried
	if derp.IsNotFoundOrGone(loadError) {
		return streams.NilDocument(), queue.Failure(derp.Wrap(loadError, location, "Recipient no longer exists", "recipient: "+recipientURL)), false
	}

	// Any other load error may pass, so the delivery is retried
	if loadError != nil {
		return streams.NilDocument(), queue.Error(derp.Wrap(loadError, location, "Loading recipient", "recipient: "+recipientURL)), false
	}

	// RULE: A recipient that is neither an actor nor a collection with an owner has no inbox
	return streams.NilDocument(), queue.Failure(derp.BadRequest(location, "Recipient is not an actor, and no owner was found for it", "recipient: "+recipientURL)), false
}

// resolveOwner applies the application's recipient test to the owner of a remote collection,
// who was not known when the fan-out filtered its list
func (pipeline Pipeline) resolveOwner(actorID string, owner streams.Document) (streams.Document, queue.Result, bool) {

	if (pipeline.filterOne != nil) && !pipeline.filterOne(actorID, owner.ID()) {
		log.Debug().Str("owner", owner.ID()).Msg("hannibal.delivery: the owner of a remote collection is excluded")
		return streams.NilDocument(), queue.Success(), false
	}

	return owner, queue.Result{}, true
}

// collectionOwner looks for the actor whose followers collection is the given URL, among the
// actors that the activity itself points to. It never reads the collection's members.
func (pipeline Pipeline) collectionOwner(collectionURL string, body string) (streams.Document, bool) {

	activity := make(map[string]any)

	if err := json.Unmarshal([]byte(body), &activity); err != nil {
		return streams.NilDocument(), false
	}

	for _, candidateURL := range pipeline.ownerCandidates(collectionURL, activity) {

		candidate, err := pipeline.client.Load(candidateURL)

		if err != nil {
			continue
		}

		// RULE: The owner must be an actor. A collection that belongs to a collection has no inbox.
		if !candidate.IsActor() {
			continue
		}

		// RULE: An actor owns only a collection on its own origin. Otherwise anyone the activity
		// names could claim another actor's followers, and take the copy meant for that actor.
		if uri.NotSameOrigin(collectionURL, candidate.ID()) {
			continue
		}

		if candidate.Followers().ID() == collectionURL {
			return candidate, true
		}
	}

	return streams.NilDocument(), false
}

// ownerCandidates lists the actors that might own a collection the activity names: the authors
// of the objects it replies to, then the people it names directly
func (pipeline Pipeline) ownerCandidates(collectionURL string, activity map[string]any) []string {

	result := make([]string, 0, maxOwnerCandidates)

	add := func(candidateURLs ...string) {
		for _, candidateURL := range candidateURLs {

			if len(result) >= maxOwnerCandidates {
				return
			}

			if (candidateURL == "") || (candidateURL == collectionURL) {
				continue
			}

			result = append(result, candidateURL)
		}
	}

	// RULE: The Document is made without a client, so reading it loads nothing
	document := streams.NewDocument(activity)

	// A reply to someone's post is the usual reason to name their followers
	var parentURLs []string

	if object := document.Object(); object.IsMap() {
		parentURLs = object.InReplyTo().IDs()
	}

	// RULE: The list of parents comes from the activity, so only the first few are loaded
	if len(parentURLs) > maxReplyParents {
		parentURLs = parentURLs[:maxReplyParents]
	}

	for _, parentURL := range parentURLs {

		parent, err := pipeline.client.Load(parentURL)

		if err != nil {
			continue
		}

		// RULE: The parent has a client, so its authors are read as IDs, which loads nothing.
		// `Actor` would load each one.
		add(parent.Get(vocab.PropertyAttributedTo).IDs()...)
		add(parent.Get(vocab.PropertyActor).IDs()...)
	}

	// Otherwise the owner is often named beside their collection
	for recipient := range extract(document) {
		if !recipient.IsPublic() {
			add(recipient.URL)
		}
	}

	return result
}

// post signs one request with the actor's key and delivers the body to the inbox as it was queued
func (pipeline Pipeline) post(signer Signer, inboxURL string, body string) error {

	transaction := remote.Post(inboxURL).
		Accept(vocab.ContentTypeActivityPub).
		ContentType(vocab.ContentTypeActivityPub).
		With(signRequest(signer.PrivateKey())).
		Body(body).
		AllowPrivateIPs(pipeline.allowPrivateIPs)

	// The application's own options, such as a circuit breaker's, are built new for each request
	if pipeline.remoteOptions != nil {
		transaction.With(pipeline.remoteOptions()...)
	}

	return transaction.Send()
}

// result returns what happens to a send task after its delivery. It only reads the error: a
// circuit breaker, if the application added one, has already recorded the outcome for itself.
func (pipeline Pipeline) result(host string, queuedAt int64, sendError error) queue.Result {

	const location = "hannibal.delivery.Pipeline.result"

	if sendError == nil {
		return queue.Success()
	}

	// RULE: A host that gets nothing is skipped quietly, and nothing is held for it. A circuit
	// breaker answers this way for a host that is dormant or paused.
	if derp.ErrorCode(sendError) == http.StatusLocked {
		return queue.Success()
	}

	// RULE: A rate limit is not a failure. The delivery is rescheduled without spending a retry,
	// for as long as the host asked. derp answers one minute when the host names no wait.
	if isRateLimited, retryAfter := derp.IsTooManyRequests(sendError); isRateLimited {

		// RULE: A reschedule spends no retry, so only its age can end a delivery to a host that
		// never stops limiting it
		if isExpired(queuedAt, time.Now()) {
			return queue.Failure(derp.Wrap(sendError, location, "Host has rate limited this delivery for too long", "host: "+host))
		}

		return queue.Requeue(requeueDelay(retryAfter))
	}

	// RULE: An inbox that is gone will not come back
	if derp.IsNotFoundOrGone(sendError) {
		return queue.Failure(derp.Wrap(sendError, location, "Inbox no longer exists", "host: "+host))
	}

	// RULE: A server error or a timeout says that the host is in trouble, so the delivery is retried
	if derp.IsServerError(sendError) || derp.IsTimeoutError(sendError) {
		return queue.Error(derp.Wrap(sendError, location, "Host could not accept the delivery", "host: "+host))
	}

	// RULE: Any other refusal is about this delivery, not about the host, and will not change on a retry
	return queue.Failure(derp.Wrap(sendError, location, "Host refused the delivery", "host: "+host))
}

// isExpired returns TRUE if a delivery was first queued longer ago than a delivery may wait.
// A task that does not say when it was queued never expires.
func isExpired(queuedAt int64, now time.Time) bool {

	if queuedAt <= 0 {
		return false
	}

	return now.Sub(time.Unix(queuedAt, 0)) > maximumDeliveryAge
}

// requeueDelay returns how long a rescheduled delivery waits, within the shortest and longest waits allowed
func requeueDelay(retryAfter time.Duration) time.Duration {
	return min(max(retryAfter, minimumRequeueDelay), maximumRequeueDelay)
}
