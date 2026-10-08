package streams

import (
	"time"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/convert"
)

/******************************************
 * ActivityStreams 2.0 Properties
 * https://www.w3.org/TR/activitystreams-vocabulary
 ******************************************/

// AtContext returns the document's AtContext property.
// https://www.w3.org/TR/activitystreams-core/#h-jsonld
func (document Document) AtContext() Document {
	return document.Get(vocab.AtContext)
}

// ID returns the document's ID property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-id
func (document Document) ID() string {

	// Try the ActivityPub standard "id" property first
	if id := document.Get(vocab.PropertyID); id.NotNil() {
		return id.String()
	}

	// Try the JSON-LD standard "@id" property second
	if id := document.Get(vocab.PropertyID_Alternate); id.NotNil() {
		return document.Get(vocab.PropertyID).String()
	}

	// LOL, no.
	return ""
}

// Actor returns the document's Actor property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-actor
func (document Document) Actor() Document {
	return document.Get(vocab.PropertyActor).LoadLink()
}

// ActorID returns just the ID of the document's Actor property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-actor
func (document Document) ActorID() string {
	return document.Get(vocab.PropertyActor).ID()
}

// Attachment returns the document's Attachment property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-attachment
func (document Document) Attachment() Document {
	return document.Get(vocab.PropertyAttachment)
}

// AttributedTo returns the document's AttributedTo property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-attributedto
func (document Document) AttributedTo() Document {
	return document.Get(vocab.PropertyAttributedTo)
}

// Audience returns the document's Audience property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-audience
func (document Document) Audience() Document {
	return document.Get(vocab.PropertyAudience)
}

// BCC returns the document's BCC property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-bcc
func (document Document) BCC() Document {
	return document.Get(vocab.PropertyBCC)
}

// BTo returns the document's BTo property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-bto
func (document Document) BTo() Document {
	return document.Get(vocab.PropertyBTo)
}

// CC returns the document's CC property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-cc
func (document Document) CC() Document {
	return document.Get(vocab.PropertyCC)
}

// Context returns the document's Context property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-context
// IMPORTANT: THIS IS NOT THE @context PROPERTY REQUIRED FOR EVERY JSON-LD DOCUMENT
func (document Document) Context() string {
	return document.Get(vocab.PropertyContext).String()
}

// Current returns the document's Current property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-current
func (document Document) Current() Document {
	return document.Get(vocab.PropertyCurrent)
}

// Deleted returns the document's Deleted property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-deleted
func (document Document) Deleted() time.Time {
	return document.Get(vocab.PropertyDeleted).Time()
}

// Describes returns the document's Describes property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-describes
func (document Document) Describes() Document {
	return document.Get(vocab.PropertyDescribes)
}

// Encoding returns the document's Encoding property.
// https://swicg.github.io/activitypub-e2ee/mls#encoding
func (document Document) Encoding() string {
	return document.Get(vocab.PropertyEncoding).String()
}

// First returns the document's First property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-first
func (document Document) First() Document {
	return document.Get(vocab.PropertyFirst)
}

// FormerType returns the document's FormerType property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-formertype
func (document Document) FormerType() string {
	return document.Get(vocab.PropertyFormerType).String()
}

// Generator returns the document's Generator property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-generator
func (document Document) Generator() Document {
	return document.Get(vocab.PropertyGenerator)
}

// Icon returns the document's Icon property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-icon
func (document Document) Icon() Image {
	return NewImage(document.Get(vocab.PropertyIcon))
}

// IconOrImage is a hybrid accessor that returns the "icon" property (if not nil),
// otherwise it returns the "image" property.  This is useful for working with different
// ActivityPub objects, which may use either property.
func (document Document) IconOrImage() Image {

	if icon := document.Icon(); icon.NotNil() {
		return icon
	}

	if image := document.Image(); image.NotNil() {
		return image
	}

	if attachment := document.FirstImageAttachment(); attachment.NotNil() {
		return attachment
	}

	return NewImage("")
}

// Image returns the document's Image property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-image
func (document Document) Image() Image {
	return NewImage(document.Get(vocab.PropertyImage))
}

// ImageOrIcon is a hybrid accessor that returns the "image" property (if not nil),
// otherwise it returns the "icon" property.  This is useful for working with different
// ActivityPub objects, which may use either property.
func (document Document) ImageOrIcon() Image {

	if image := document.Image(); image.NotNil() {
		return image
	}

	if attachment := document.FirstImageAttachment(); attachment.NotNil() {
		return attachment
	}

	if icon := document.Icon(); icon.NotNil() {
		return icon
	}

	return NewImage("")
}

// InReplyTo returns the document's InReplyTo property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-inreplyto
func (document Document) InReplyTo() Document {
	return document.Get(vocab.PropertyInReplyTo)
}

// Instrument returns the document's Instrument property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-instrument
func (document Document) Instrument() Document {
	return document.Get(vocab.PropertyInstrument)
}

// Items returns the items collection for this Document.  If the
// document contains an "orderedItems" collection, then it is
// returned instead.
func (document Document) Items() Document {

	// Search the "orderedItems" property first (guessing this will be more common)
	if result := document.Get(vocab.PropertyOrderedItems); !result.IsNil() {
		return result
	}

	// Search the "items" property second (guessing this will be less common)
	if result := document.Get(vocab.PropertyItems); !result.IsNil() {
		return result
	}

	// Value not found :(
	return NilDocument()
}

// Last returns the document's Last property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-last
func (document Document) Last() Document {
	return document.Get(vocab.PropertyLast)
}

// Location returns the document's Location property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-location
func (document Document) Location() Document {
	return document.Get(vocab.PropertyLocation)
}

// MLSCiphersuite returns the document's MLSCiphersuite property.
// https://swicg.github.io/activitypub-e2ee/mls#ciphersuite
func (document Document) MLSCiphersuite() string {
	return document.Get(vocab.PropertyMLSCiphersuite).String()
}

// MLSKeyPackages returns the document's MLSKeyPackages property.
// https://swicg.github.io/activitypub-e2ee/mls#keyPackages
func (document Document) MLSKeyPackages() Document {
	return document.Get(vocab.PropertyMLSKeyPackages)
}

// OneOf returns the document's OneOf property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-oneof
func (document Document) OneOf() Document {
	return document.Get(vocab.PropertyOneOf)
}

// AnyOf returns the document's AnyOf property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-anyof
func (document Document) AnyOf() Document {
	return document.Get(vocab.PropertyAnyOf)
}

// Closed returns the document's Closed property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-closed
func (document Document) Closed() Document {
	return document.Get(vocab.PropertyClosed)
}

// Origin returns the document's Origin property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-origin
func (document Document) Origin() Document {
	return document.Get(vocab.PropertyOrigin)
}

// Next returns the document's Next property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-next
func (document Document) Next() Document {
	return document.Get(vocab.PropertyNext)
}

// Object returns the document's Object property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-object
func (document Document) Object() Document {
	return document.Get(vocab.PropertyObject)
}

// Prev returns the document's Prev property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-prev
func (document Document) Prev() Document {
	return document.Get(vocab.PropertyPrev)
}

// Preview returns the document's Preview property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-preview
func (document Document) Preview() Document {
	return document.Get(vocab.PropertyPreview)
}

// PublicKey returns the document's PublicKey property.
func (document Document) PublicKey() Document {
	return document.Get(vocab.PropertyPublicKey)
}

// PublicKeyPEM returns the document's PublicKeyPEM property.
func (document Document) PublicKeyPEM() string {
	return document.Get(vocab.PropertyPublicKeyPEM).String()
}

// Result returns the document's Result property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-result
func (document Document) Result() Document {
	return document.Get(vocab.PropertyResult)
}

// Replies returns the document's Replies property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-replies
func (document Document) Replies() Document {
	return document.Get(vocab.PropertyReplies)
}

// SharedInbox returns the document's SharedInbox property.
// https://www.w3.org/TR/activitypub/#sharedInbox
func (document Document) SharedInbox() string {
	return document.Get(vocab.EndpointSharedInbox).String()
}

// Shares returns the document's Shares property.
// https://www.w3.org/TR/activitypub/#shares
func (document Document) Shares() Document {
	return document.Get(vocab.PropertyShares)
}

// Tag returns the document's Tag property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-tag
func (document Document) Tag() Document {
	return document.Get(vocab.PropertyTag)
}

// Target returns the document's Target property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-target
func (document Document) Target() Document {
	return document.Get(vocab.PropertyTarget)
}

// To returns the document's To property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-to
func (document Document) To() Document {
	return document.Get(vocab.PropertyTo)
}

// Type returns the document's Type property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-type
func (document Document) Type() string {

	// Try the ActivityPub standard "type" property first
	if value := document.Get(vocab.PropertyType); !value.IsNil() {
		return value.String()
	}

	// Try the JSON-LD standard "@type" property second
	if value := document.Get(vocab.PropertyType_Alternate); !value.IsNil() {
		return value.String()
	}

	// LOL, Fail
	return vocab.Unknown
}

// Types returns the document's Types property.
// A special case of the Type() function, which returns a slice of types
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-type
func (document Document) Types() []string {

	// Try the ActivityPub standard "type" property first
	if value := document.Get(vocab.PropertyType); !value.IsNil() {
		return convert.SliceOfString(value.Slice())
	}

	// Try the JSON-LD standard "@type" property second
	if value := document.Get(vocab.PropertyType_Alternate); !value.IsNil() {
		return convert.SliceOfString(value.Slice())
	}

	// LOL, Fail
	return []string{vocab.Unknown}
}

// Accuracy returns the document's Accuracy property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-accuracy
func (document Document) Accuracy() float64 {
	return document.Get(vocab.PropertyAccuracy).Float()
}

// Altitude returns the document's Altitude property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-altitude
func (document Document) Altitude() float64 {
	return document.Get(vocab.PropertyAltitude).Float()
}

// Content returns the document's Content property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-content
func (document Document) Content() string {
	return document.Get(vocab.PropertyContent).HTMLString()
}

// Name returns the document's Name property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-name
func (document Document) Name() string {
	// Language maps are not yet resolved here; the raw string value is returned.
	return document.Get(vocab.PropertyName).String()
}

// Duration returns the document's Duration property. ISO 8601 duration parsing per
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-duration is not yet implemented.
func (document Document) Duration() string {

	return document.Get(vocab.PropertyDuration).String()
}

// Height returns the document's Height property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-height
func (document Document) Height() int {
	return document.Get(vocab.PropertyHeight).Int()
}

// Href returns the document's Href property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-href
func (document Document) Href() string {
	return document.Get(vocab.PropertyHref).String()
}

// Hreflang returns the document's Hreflang property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-hreflang
func (document Document) Hreflang() string {
	return document.Get(vocab.PropertyHrefLang).String()
}

// PartOf returns the document's PartOf property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-partof
func (document Document) PartOf() Document {
	return document.Get(vocab.PropertyPartOf)
}

// Latitude returns the document's Latitude property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-latitude
func (document Document) Latitude() float64 {
	return document.Get(vocab.PropertyLatitude).Float()
}

// Longitude returns the document's Longitude property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-longitude
func (document Document) Longitude() float64 {
	return document.Get(vocab.PropertyLongitude).Float()
}

// MediaType returns the document's MediaType property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-mediatype
func (document Document) MediaType() string {
	return document.Get(vocab.PropertyMediaType).String()
}

// EndTime returns the document's EndTime property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-endtime
func (document Document) EndTime() time.Time {
	return document.Get(vocab.PropertyEndTime).Time()
}

// Published returns the document's Published property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-published
func (document Document) Published() time.Time {
	return document.Get(vocab.PropertyPublished).Time()
}

// StartTime returns the document's StartTime property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-starttime
func (document Document) StartTime() time.Time {
	return document.Get(vocab.PropertyStartTime).Time()
}

// Radius returns the document's Radius property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-radius
func (document Document) Radius() float64 {
	return document.Get(vocab.PropertyRadius).Float()
}

// Rel returns the document's Rel property. It is expected to be a string, but this
// function returns a Document because it may contain multiple values (rel:["canonical", "preview"]).
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-rel
func (document Document) Rel() Document {
	return document.Get(vocab.PropertyRel)
}

// Relationship returns the document's Relationship property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-relationship
func (document Document) Relationship() string {
	return document.Get(vocab.PropertyRelationship).String()
}

// StartIndex returns the document's StartIndex property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-startindex
func (document Document) StartIndex() int {
	return document.Get(vocab.PropertyStartIndex).Int()
}

// Subject returns the document's Subject property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-subject
func (document Document) Subject() Document {
	return document.Get(vocab.PropertySubject)
}

// Summary returns the document's Summary property. Language maps are not yet resolved here.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-summary
func (document Document) Summary() string {
	return document.Get(vocab.PropertySummary).String()
}

// TotalItems returns the document's TotalItems property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-totalitems
func (document Document) TotalItems() int {
	return document.Get(vocab.PropertyTotalItems).Int()
}

// URL returns the document's URL property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-url
func (document Document) URL() string {

	url := document.Get(vocab.PropertyURL)

	// A list of Links names the profile page with its text/html Link, as Mastodon reads it
	for link := range url.Range() {
		if link.IsMap() && (link.MediaType() == "text/html") {
			return link.Href()
		}
	}

	// Otherwise, use the first value: a string as it is, or a Link's href
	first := url.Head()

	if first.IsMap() {
		return first.Href()
	}

	return first.String()
}

// URLOrID returns the URL of the document, if it exists, otherwise it returns the ID.
func (document Document) URLOrID() string {
	if url := document.URL(); url != "" {
		return url
	}
	return document.ID()
}

// Units returns the document's Units property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-units
func (document Document) Units() string {
	return document.Get(vocab.PropertyUnits).String()
}

// Updated returns the document's Updated property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-updated
func (document Document) Updated() time.Time {
	return document.Get(vocab.PropertyUpdated).Time()
}

// Width returns the document's Width property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-width
func (document Document) Width() int {
	return document.Get(vocab.PropertyWidth).Int()
}
