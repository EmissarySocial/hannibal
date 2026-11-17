// Package sender provides an object and queue consumer that
// can send outbound ActivityPub activities from an outbox
// to actor's inbox URLs. It automatically signs activities
// using the sending actor's private key.
package sender

import (
	"encoding/json"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/remote"
	"github.com/benpate/remote/options"
	"github.com/benpate/rosetta/convert"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/ranges"
	"github.com/benpate/turbine/queue"
	"github.com/rs/zerolog/log"
)

// Sender manages delivery of outbound activities from the outbox,
// using the turbine Queue to deliver activities asynchronously.
type Sender struct {
	queue           *queue.Queue // Queue processes messages asynchronously
	locator         Locator      // Locator resolves Actor IDs into Actor objects, and resolves recipient URIs into inbox URLs.
	allowPrivateIPs bool         // If TRUE, outbound deliveries may connect to non-public (private/loopback) addresses.
}

// New returns a fully initialized Sender object
func New(locator Locator, q *queue.Queue, options ...Option) Sender {

	const location = "hannibal.sender.New"

	// If we don't have an actual queue, then create one
	if q == nil {
		derp.Report(
			derp.Internal(
				location,
				"A background queue was not provided to the Outbox service.",
				"This configuration is okay for testing, but should not be used in production code.",
				"A proper background queue is required to recover from failed message deliveries.",
				"Yes, you. I'm talking to you. Don't do this in production.",
			),
		)

		// Make our own in-memory queue
		// This is bad bad bad for production systems
		q = queue.New()
	}

	// Build the Sender object
	sender := Sender{
		queue:   q,
		locator: locator,
	}

	// Apply any caller-provided options
	for _, option := range options {
		option(&sender)
	}

	return sender
}

// Send queues a new task to deliver the provided activity to all recipients.
// IMPORTANT: The queue.Consumer in this package MUST be connected to a live
// queue process in order for outbound activities to be sent.
func (sender Sender) Send(activity mapof.Any) error {

	const location = "hannibal.sender.Send"

	// NILCHECK: If the Outbox was not properly initialized with a queue, then report an error and return
	if sender.queue == nil {
		return derp.Internal(
			location,
			"Message cannot be sent because the background queue was not provided.",
			"This should never happen.",
		)
	}

	// Queue a new task to send this activity to all recipients.
	task := queue.NewTask(OutboxSendToAllRecipients, activity)

	if err := sender.queue.Publish(task); err != nil {
		return derp.Wrap(err, location, "Unable to enqueue outbound activity", activityName(activity))
	}

	// Success!
	return nil
}

// SendToAllRecipients resolves every recipient of the provided ActivityPub activity into
// inbox URLs, then enqueues a separate SendToSingleRecipient task for each one.
func (sender *Sender) SendToAllRecipients(activity mapof.Any) queue.Result {

	const location = "hannibal.sender.SendToAllRecipients"

	// Locate the Actor that is sending this activity
	actorID := activity.GetString(vocab.PropertyActor)
	actor, err := sender.locator.Actor(actorID)

	if err != nil {
		return actorResult(err, location, activityName(activity))
	}

	// Use the Locator to resolve recipient URIs into inbox URLs
	recipients, err := getRecipients(sender.locator, activity)

	if err != nil {
		return queue.Error(derp.Wrap(err, location, "Unable to retrieve recipient addresses", activityName(activity)))
	}

	// Remove duplicate recipient inbox URLs
	recipients = ranges.Unique(recipients)

	// RULE: No recipient may see who was blind-copied. An embedded object can name blind
	// recipients of its own, so `bto` and `bcc` are removed at every level, from a copy.
	stripped, err := hannibal.StripBlindRecipients(activity)

	if err != nil {
		return queue.Failure(derp.Wrap(err, location, "Unable to remove blind recipients", activityName(activity)))
	}

	// Serialize the activity once, so every recipient's task shares the same string
	serialized, err := json.Marshal(stripped)

	if err != nil {
		return queue.Failure(derp.Wrap(err, location, "Unable to serialize outbound activity", activityName(activity)))
	}

	body := string(serialized)
	queuedAt := time.Now().Unix()

	// Enqueue additional tasks to send this Activity to each recipient's inboxURL
	for recipient := range recipients {

		// Skip empty inbox URLs
		if recipient == "" {
			continue
		}

		log.Debug().Str("actorID", actorID).Str("recipient", recipient).Msg("Queueing outbound activity")

		task := queue.NewTask(OutboxSendToSingleRecipient, mapof.Any{
			"actor": actor.ActorID(),
			"inbox": recipient,
			"body":  body,

			// When the delivery was first queued, which outlasts a reschedule
			argumentQueuedAt: queuedAt,
		})

		if err := sender.queue.Publish(task); err != nil {
			return queue.Error(derp.Wrap(err, location, "Unable to enqueue outbound activity", "recipient", recipient))
		}
	}

	// Task Succeeded Successfully!
	return queue.Success()
}

// SendToSingleRecipient sends a single ActivityPub activity from the provided Actor to a single recipient's inbox URL.
func (sender *Sender) SendToSingleRecipient(args mapof.Any) queue.Result {

	const location = "hannibal.sender.SendToSingleRecipient"

	// Collect arguments
	actorID := convert.String(args["actor"])
	inboxURL := convert.String(args["inbox"])

	log.Debug().Str("actorID", actorID).Str("inboxURL", inboxURL).Msg("Sending outbound activity")

	// Locate the Actor that is sending this activity
	actor, err := sender.locator.Actor(actorID)

	if err != nil {
		return actorResult(err, location, "actorID: "+actorID)
	}

	// Prepare a transaction to send to target Actor's inbox
	transaction := remote.Post(inboxURL).
		Accept(vocab.ContentTypeActivityPub).
		ContentType(vocab.ContentTypeActivityPub).
		With(signRequest(actor.PrivateKey()))

	// RULE: A task must carry something to deliver, either as "body" or as a legacy "activity"
	if !withRequestBody(transaction, args) {
		return queue.Failure(derp.Internal(location, "Task has neither a body nor an activity to send", "inboxURL: "+inboxURL))
	}

	// RULE: By default, remote refuses to connect to non-public (private/loopback)
	// addresses to guard against SSRF. sender.allowPrivateIPs stays FALSE in production;
	// callers delivering to a local peer opt in via the AllowPrivateIPs option.
	transaction.AllowPrivateIPs(sender.allowPrivateIPs)

	// Enable debugging (if requested)
	if canDebug() {
		transaction.With(options.Debug())
	}

	// Send the transaction to the recipient's inbox.
	// Errors will be handled by the asQueueResult() function in the queue.Consumer.
	if err := transaction.Send(); err != nil {

		// Special handling for HTTP 429 (Too Many Requests) error
		if tooManyRequests, retryDuration := derp.IsTooManyRequests(err); tooManyRequests {

			// RULE: A reschedule spends no retry, so only its age can end a delivery to a host
			// that never stops limiting it
			if isExpired(args.GetInt64(argumentQueuedAt), time.Now()) {
				return queue.Failure(derp.Wrap(err, location, "Host has rate limited this delivery for too long", "inboxURL: "+inboxURL))
			}

			return queue.Requeue(requeueDelay(retryDuration))
		}

		// If this is our fault then it can't be retried. Fail accordingly.
		if derp.IsClientError(err) {
			return queue.Failure(derp.Wrap(err, location, "Unable to send HTTP request (Client Error cannot be retried)"))
		}

		// Otherwise, it is a server error that can be retried by the standard queue mechanism.
		return queue.Error(derp.Wrap(err, location, "Unable to send HTTP request (Server Error can be retried)"))

	}

	// No error means the transaction was successful.  Woot woot!
	return queue.Success()
}

// withRequestBody sets the transaction's body from the task arguments,
// returning FALSE when the task carries nothing to send.
func withRequestBody(transaction *remote.Transaction, args mapof.Any) bool {

	// Send the pre-serialized body exactly as the producer wrote it
	if body := convert.String(args["body"]); body != "" {
		transaction.Body(body)
		return true
	}

	// Tasks queued before the "body" argument existed carry the activity map instead
	activity := convert.MapOfAny(args["activity"])

	if len(activity) == 0 {
		return false
	}

	transaction.JSON(activity)
	return true
}
