package sender

import (
	"iter"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/ranges"
	"github.com/benpate/turbine/queue"
	"github.com/rs/zerolog"
)

// getRecipients retrieves the inbox URLs for all recipients of this activity.
// It uses the Locator service to resolve each URI in the to, cc, bto, and bcc fields to
// one or more inbox URLs.  For example, a URI may point to a list of followers, in which
// case every follower's inbox URL will be included in the resulting iterator.
func getRecipients(locator Locator, activity mapof.Any) (iter.Seq[string], error) {

	const location = "hannibal.sender.getRecipients"

	iterators := make([]iter.Seq[string], 0)
	properties := []string{vocab.PropertyTo, vocab.PropertyCC, vocab.PropertyBTo, vocab.PropertyBCC}

	// Loop through each property
	for _, property := range properties {

		// Loop through URIs in each property
		for _, recipient := range activity.GetSliceOfString(property) {

			// Get the iterator of inboxes for this URI
			iterator, err := locator.Recipient(recipient)

			if err != nil {
				return nil, derp.Wrap(err, location, "Unable to resolve recipient for url", recipient)
			}

			// Add this iterator to the list we're going to return
			iterators = append(iterators, iterator)
		}
	}

	// Combine all results into a single iterator
	return ranges.Join(iterators...), nil
}

// canDebug returns TRUE if zerolog is configured to allow Debug logs
func canDebug() bool {
	return canLog(zerolog.DebugLevel)
}

// canLog is a silly zerolog helper that returns TRUE
// if the provided log level would be allowed
// (based on the global log level).
// This makes it easier to execute expensive code conditionally,
// for instance: marshalling a JSON object for logging.
func canLog(level zerolog.Level) bool {
	return zerolog.GlobalLevel() <= level
}

// activityName returns the detail that names an activity in an error.
// RULE: An error never carries the activity itself, which holds its blind recipients.
func activityName(activity mapof.Any) string {
	return "activityID: " + activity.GetString(vocab.PropertyID) + ", actorID: " + activity.GetString(vocab.PropertyActor)
}

// actorResult returns what happens to a task whose sending actor could not be loaded
func actorResult(err error, location string, detail string) queue.Result {

	wrapped := derp.Wrap(err, location, "Unable to load the sending actor", detail)

	// RULE: An actor that does not exist will not come back, so the task ends
	if derp.IsNotFoundOrGone(err) {
		return queue.Failure(wrapped)
	}

	// Any other error, such as a database that did not answer, may pass
	return queue.Error(wrapped)
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
