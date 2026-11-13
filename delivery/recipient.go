package delivery

import (
	"net/url"
	"strings"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/uri"
)

// Recipient is one address taken from an activity, with the property that named it
type Recipient struct {
	URL   string // The address as the activity wrote it: an actor, a collection, or the Public marker
	Field string // The property it came from: FieldTo, FieldCC, FieldBTo, FieldBCC, or FieldRelay
}

// IsPublic returns TRUE if this recipient is the Public marker, in any of its three spellings
func (recipient Recipient) IsPublic() bool {

	switch recipient.URL {
	case vocab.NamespaceActivityStreamsPublic, vocab.NamespaceASPublic, vocab.NamespacePublic:
		return true
	}

	return false
}

// IsEmail returns TRUE if a recipient URL names someone who receives the activity by email
func IsEmail(recipientURL string) bool {
	return strings.HasPrefix(recipientURL, schemeMailto)
}

// HostOf returns the hostname of a URL in lower case, without its port, and an empty string
// for anything that is not a URL with a host. It is the key that a host's health is kept under.
func HostOf(value string) string {

	parsed, err := url.Parse(value)

	if err != nil {
		return ""
	}

	if parsed.Host == "" {
		return ""
	}

	// One function names a host everywhere, so that every package keys the same host the same way
	return uri.Hostname(value)
}
