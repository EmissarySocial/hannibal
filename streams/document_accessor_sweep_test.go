package streams

import (
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
)

// TestDocument_AccessorSweep_Documents exercises the thin Document-returning
// property accessors. Each is a Get(<property>) wrapper; we populate a document
// with a distinct id per property and confirm the accessor descends to it.
func TestDocument_AccessorSweep_Documents(t *testing.T) {

	// id builds a sub-object whose id encodes the property name, so we can assert
	// the accessor reached the right key.
	id := func(name string) map[string]any {
		return map[string]any{vocab.PropertyID: "urn:" + name}
	}

	doc := NewDocument(map[string]any{
		vocab.AtContext:          id("context"),
		vocab.PropertyActor:      id("actor"),
		vocab.PropertyAttachment: id("attachment"),
		vocab.PropertyAudience:   id("audience"),
		vocab.PropertyBCC:        id("bcc"),
		vocab.PropertyBTo:        id("bto"),
		vocab.PropertyCC:         id("cc"),
		vocab.PropertyCurrent:    id("current"),
		vocab.PropertyDescribes:  id("describes"),
		vocab.PropertyFirst:      id("first"),
		vocab.PropertyGenerator:  id("generator"),
		vocab.PropertyInReplyTo:  id("inReplyTo"),
		vocab.PropertyInstrument: id("instrument"),
		vocab.PropertyItems:      id("items"),
		vocab.PropertyLast:       id("last"),
		vocab.PropertyLocation:   id("location"),
		vocab.PropertyNext:       id("next"),
		vocab.PropertyOrigin:     id("origin"),
		vocab.PropertyPrev:       id("prev"),
		vocab.PropertyPreview:    id("preview"),
		vocab.PropertyShares:     id("shares"),
		vocab.PropertyTarget:     id("target"),

		// Actor collection properties (document_actor.go).
		vocab.PropertyInbox:     id("inbox"),
		vocab.PropertyOutbox:    id("outbox"),
		vocab.PropertyFollowing: id("following"),
		vocab.PropertyFollowers: id("followers"),
		vocab.PropertyLiked:     id("liked"),
		vocab.PropertyLikes:     id("likes"),
		vocab.PropertyBlocked:   id("blocked"),
		vocab.PropertyStreams:   id("streams"),
		vocab.PropertyFeatured:  id("featured"),
		vocab.PropertyEndpoints: id("endpoints"),
	})

	// check asserts that the accessor descended to the sub-object keyed by name.
	check := func(name string, accessor func(Document) Document) {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "urn:"+name, accessor(doc).ID())
		})
	}

	check("context", Document.AtContext)
	check("actor", Document.Actor)
	check("attachment", Document.Attachment)
	check("audience", Document.Audience)
	check("bcc", Document.BCC)
	check("bto", Document.BTo)
	check("cc", Document.CC)
	check("current", Document.Current)
	check("describes", Document.Describes)
	check("first", Document.First)
	check("generator", Document.Generator)
	check("inReplyTo", Document.InReplyTo)
	check("instrument", Document.Instrument)
	check("items", Document.Items)
	check("last", Document.Last)
	check("location", Document.Location)
	check("next", Document.Next)
	check("origin", Document.Origin)
	check("prev", Document.Prev)
	check("preview", Document.Preview)
	check("shares", Document.Shares)
	check("target", Document.Target)

	check("inbox", Document.Inbox)
	check("outbox", Document.Outbox)
	check("following", Document.Following)
	check("followers", Document.Followers)
	check("liked", Document.Liked)
	check("likes", Document.Likes)
	check("blocked", Document.Blocked)
	check("streams", Document.Streams)
	check("featured", Document.Featured)
	check("endpoints", Document.Endpoints)
}

// TestDocument_AccessorSweep_Strings exercises the thin string-returning
// property accessors.
func TestDocument_AccessorSweep_Strings(t *testing.T) {

	doc := NewDocument(map[string]any{
		vocab.PropertyContext:      "context-value",
		vocab.PropertyEncoding:     "encoding-value",
		vocab.PropertyFormerType:   "formerType-value",
		vocab.PropertyDuration:     "PT5M",
		vocab.PropertyHrefLang:     "en-us",
		vocab.PropertyRelationship: "relationship-value",
		vocab.PropertyUnits:        "units-value",
	})

	check := func(name string, expected string, accessor func(Document) string) {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, accessor(doc))
		})
	}

	check("Context", "context-value", Document.Context)
	check("Encoding", "encoding-value", Document.Encoding)
	check("FormerType", "formerType-value", Document.FormerType)
	check("Duration", "PT5M", Document.Duration)
	check("Hreflang", "en-us", Document.Hreflang)
	check("Relationship", "relationship-value", Document.Relationship)
	check("Units", "units-value", Document.Units)
}

// TestDocument_AccessorSweep_Numbers exercises the thin numeric accessors.
func TestDocument_AccessorSweep_Numbers(t *testing.T) {

	doc := NewDocument(map[string]any{
		vocab.PropertyAccuracy:   99.5,
		vocab.PropertyAltitude:   1200.0,
		vocab.PropertyRadius:     50.0,
		vocab.PropertyStartIndex: 10,
		vocab.PropertyTotalItems: 42,
	})

	assert.Equal(t, 99.5, doc.Accuracy())
	assert.Equal(t, 1200.0, doc.Altitude())
	assert.Equal(t, 50.0, doc.Radius())
	assert.Equal(t, 10, doc.StartIndex())
	assert.Equal(t, 42, doc.TotalItems())
}

// TestDocument_UsernameOrID confirms UsernameOrID builds a fediverse handle when
// a username is present, and falls back to the id otherwise.
func TestDocument_UsernameOrID(t *testing.T) {

	withName := NewDocument(map[string]any{
		vocab.PropertyID:                "https://example.com/users/alice",
		vocab.PropertyPreferredUsername: "alice",
	})
	assert.Equal(t, "@alice@example.com", withName.UsernameOrID())

	// No username -> falls back to the raw id.
	idOnly := NewDocument(map[string]any{vocab.PropertyID: "https://example.com/users/bob"})
	assert.Equal(t, "https://example.com/users/bob", idOnly.UsernameOrID())
}
