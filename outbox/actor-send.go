package outbox

import (
	"iter"
	"net/url"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/delivery"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/remote"
	"github.com/benpate/remote/options"
	"github.com/benpate/rosetta/mapof"
)

/******************************************
 * Sending Messages
 ******************************************/

// Send delivers a message to every recipient in the iterators, once each.
// https://www.w3.org/TR/activitypub/#delivery
func (actor *Actor) Send(message mapof.Any, recipients ...iter.Seq[string]) {

	const location = "hannibal.outbox.actor.Send"

	// RULE: No recipient may see who was blind-copied, so one stripped copy is sent to everyone
	stripped, err := delivery.StripBlindRecipients(message)

	if err != nil {
		derp.Report(derp.Wrap(err, location, "Unable to remove blind recipients"))
		return
	}

	// RULE: A recipient named more than once receives one copy
	sent := make(map[string]struct{})

	// Send the message to each recipient
	for _, iterator := range recipients {

		for recipientID := range iterator {

			// Don't send to empty recipients
			if recipientID == "" {
				continue
			}

			// Don't send to the magic public recipient, in any of its spellings
			if (delivery.Recipient{URL: recipientID}).IsPublic() {
				continue
			}

			// Don't send messages to myself
			if recipientID == actor.actorID {
				continue
			}

			if _, alreadySent := sent[recipientID]; alreadySent {
				continue
			}

			sent[recipientID] = struct{}{}

			if err := actor.sendStripped(recipientID, stripped); err != nil {
				derp.Report(derp.Wrap(err, location, "Unable to send message", recipientID))
			}
		}
	}
}

// SendOne sends a single message to a single recipient
func (actor *Actor) SendOne(recipientID string, message mapof.Any) error {

	// RULE: No recipient may see who was blind-copied, at any level of the message
	stripped, err := delivery.StripBlindRecipients(message)

	if err != nil {
		return derp.Wrap(err, "hannibal.outbox.actor.SendOne", "Unable to remove blind recipients", recipientID)
	}

	return actor.sendStripped(recipientID, stripped)
}

// sendStripped delivers a message that holds no blind recipients to one recipient's inbox
func (actor *Actor) sendStripped(recipientID string, message map[string]any) error {

	const location = "hannibal.outbox.actor.sendStripped"

	// Use the recipientID to look up their inbox URL
	recipient := streams.NewDocument(recipientID, streams.WithClient(actor.getClient()))
	recipient, err := recipient.Load()

	if err != nil {
		return derp.Wrap(err, location, "Unable to load recipient", recipientID)
	}

	inboxURL := recipient.Inbox().ID()

	// RULE: Only a recipient with an inbox can be sent anything. A collection has none.
	if inboxURL == "" {
		return derp.BadRequest(location, "Recipient has no inbox", recipientID)
	}

	// RULE: InboxURL must be a valid URL
	inbox, err := url.Parse(inboxURL)

	if err != nil {
		return derp.Wrap(err, location, "Invalid Inbox URL", inboxURL)
	}

	// Prepare a transaction to send to target Actor's inbox
	transaction := remote.Post(inbox.String()).
		Accept(vocab.ContentTypeActivityPub).
		ContentType(vocab.ContentTypeActivityPub).
		With(SignRequest(*actor)).
		JSON(message)

	// RULE: By default, remote refuses to connect to non-public (private/loopback)
	// addresses to guard against SSRF. The Actor's allowPrivateIPs flag stays FALSE
	// in production; callers delivering to a local peer opt in via WithAllowPrivateIPs.
	transaction.AllowPrivateIPs(actor.allowPrivateIPs)

	if canDebug() {
		transaction.With(options.Debug())
	}

	// Send the transaction
	if err := transaction.Send(); err != nil {
		return derp.Wrap(err, location, "Unable to send ActivityPub request", inboxURL)
	}

	return nil
}
