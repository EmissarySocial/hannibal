package delivery

import (
	"iter"

	"github.com/rs/zerolog/log"
)

/******************************************
 * Pipeline Stages
 *
 * Each stage takes an iterator and returns an iterator, so that
 * the stages stack and a recipient flows through all of them
 * before the next one is read. No stage loads anything from the
 * network, and no stage feeds an earlier one.
 ******************************************/

// Collections expands the collections that belong to the application, such as an actor's
// followers or one of its circles
type Collections interface {

	// IsCollection returns TRUE if a URL names one of the application's own collections. It must
	// decide from the URL alone, without a lookup.
	IsCollection(collectionURL string) bool

	// Members returns the members of one of the application's own collections: an actor URL for
	// each member who receives ActivityPub deliveries, and a mailto: URL for each who receives email
	Members(collectionURL string) (iter.Seq[string], error)
}

// HostList is a set of hosts that nothing should be queued for
type HostList interface {
	Contains(host string) bool
}

// addressingFields lists the properties that name an activity's recipients, in the order they are read
var addressingFields = []string{FieldTo, FieldCC, FieldBTo, FieldBCC}

// Extract yields every recipient that an activity names in `to`, `cc`, `bto` and `bcc`, with the
// property it came from. It reads the activity only: nothing is looked up, expanded or dropped.
func Extract(activity map[string]any) iter.Seq[Recipient] {

	return func(yield func(Recipient) bool) {

		for _, field := range addressingFields {
			for _, recipientURL := range referenceIDs(activity[field]) {
				if !yield(Recipient{URL: recipientURL, Field: field}) {
					return
				}
			}
		}
	}
}

// ExpandRelays replaces the Public marker with the relays that receive public activities. A
// marker in `to` yields every relay, once; a marker anywhere else is dropped, because an
// unlisted activity goes to no relay. Every other recipient passes through.
func ExpandRelays(recipients iter.Seq[Recipient], relays iter.Seq[string]) iter.Seq[Recipient] {

	return func(yield func(Recipient) bool) {

		relaysSent := false

		for recipient := range recipients {

			if !recipient.IsPublic() {
				if !yield(recipient) {
					return
				}
				continue
			}

			// RULE: The Public marker is never a place to deliver to
			if (recipient.Field != FieldTo) || relaysSent {
				continue
			}

			relaysSent = true

			for relayURL := range relays {
				if !yield(Recipient{URL: relayURL, Field: FieldRelay}) {
					return
				}
			}
		}
	}
}

// ExpandCollections replaces each of the application's own collections with its members, one
// level deep, and passes every other recipient through as its URL. A collection found inside a
// collection is dropped. The first error from a collection is handed to onError, and that
// collection yields nothing.
func ExpandCollections(recipients iter.Seq[Recipient], collections Collections, onError func(error)) iter.Seq[string] {

	return func(yield func(string) bool) {

		for recipient := range recipients {

			if !collections.IsCollection(recipient.URL) {
				if !yield(recipient.URL) {
					return
				}
				continue
			}

			members, err := collections.Members(recipient.URL)

			if err != nil {
				onError(err)
				continue
			}

			for member := range members {

				// RULE: Expansion goes one level deep, so that no collection can loop back on itself
				if collections.IsCollection(member) {
					log.Warn().Str("collection", recipient.URL).Str("member", member).Msg("hannibal.delivery: dropped a collection nested inside a collection")
					continue
				}

				if !yield(member) {
					return
				}
			}
		}
	}
}

// Deduplicate yields each URL the first time it appears, so that someone who is in several
// collections is sent one copy
func Deduplicate(recipientURLs iter.Seq[string]) iter.Seq[string] {

	return func(yield func(string) bool) {

		seen := make(map[string]struct{})

		for recipientURL := range recipientURLs {

			if recipientURL == "" {
				continue
			}

			if _, duplicate := seen[recipientURL]; duplicate {
				continue
			}

			seen[recipientURL] = struct{}{}

			if !yield(recipientURL) {
				return
			}
		}
	}
}

// SkipBlockedHosts drops every recipient whose host is in the list. A recipient who receives
// email passes through, because the list describes ActivityPub hosts.
func SkipBlockedHosts(recipientURLs iter.Seq[string], blocked HostList) iter.Seq[string] {

	return func(yield func(string) bool) {

		for recipientURL := range recipientURLs {

			if !IsEmail(recipientURL) && blocked.Contains(HostOf(recipientURL)) {
				continue
			}

			if !yield(recipientURL) {
				return
			}
		}
	}
}

// Filter yields only the recipients that the application allows. It runs once, after the
// duplicates are gone, so that each recipient is checked a single time.
func Filter(recipientURLs iter.Seq[string], allowed func(recipientURL string) bool) iter.Seq[string] {

	return func(yield func(string) bool) {

		for recipientURL := range recipientURLs {

			if !allowed(recipientURL) {
				continue
			}

			if !yield(recipientURL) {
				return
			}
		}
	}
}

// referenceIDs returns the ids that a property value names, without loading anything. The
// value may be one reference or a list, and each reference may be a URL or an object with an id.
func referenceIDs(value any) []string {

	switch typed := value.(type) {

	case nil:
		return nil

	case string:
		return []string{typed}

	case []string:
		return typed

	case map[string]any:
		if id, isString := typed["id"].(string); isString {
			return []string{id}
		}
		return nil
	}

	// Any other shape goes through the same copy that removes blind recipients, which turns
	// every named map and slice type into a plain one
	switch plain := stripValue(value).(type) {

	case map[string]any:
		return referenceIDs(plain)

	case []any:
		result := make([]string, 0, len(plain))
		for _, item := range plain {
			result = append(result, referenceIDs(item)...)
		}
		return result
	}

	return nil
}
