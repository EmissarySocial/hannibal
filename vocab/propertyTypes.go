package vocab

/******************************************
 * Standard Property Types
 ******************************************/

// AtContext is the "@context" property.
// JSON-LD context descriptor used by ActivityStreams/ActivityPub
const AtContext = "@context"

// PropertyAccuracy is the "accuracy" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-accuracy
const PropertyAccuracy = "accuracy"

// PropertyActor is the "actor" property.
// https:// www.w3.org/TR/activitystreams-vocabulary/#dfn-actor
const PropertyActor = "actor"

// PropertyAltitude is the "altitude" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-altitude
const PropertyAltitude = "altitude"

// PropertyAnyOf is the "anyOf" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-anyof
const PropertyAnyOf = "anyOf"

// PropertyAttachment is the "attachment" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-attachment
const PropertyAttachment = "attachment"

// PropertyAttributedTo is the "attributedTo" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-attributedto
const PropertyAttributedTo = "attributedTo"

// PropertyAudience is the "audience" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-audience
const PropertyAudience = "audience"

// PropertyBCC is the "bcc" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-bcc
const PropertyBCC = "bcc"

// PropertyBlurHash is the "blurHash" property.
// https://docs.joinmastodon.org/spec/activitypub/#blurhash
const PropertyBlurHash = "blurHash"

// PropertyBTo is the "bto" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-bto
const PropertyBTo = "bto"

// PropertyCC is the "cc" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-cc
const PropertyCC = "cc"

// PropertyClosed is the "closed" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-closed
const PropertyClosed = "closed"

// PropertyContent is the "content" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-content
const PropertyContent = "content"

// DELTA: Emissary I18N §1.8 (ActivityPub): new constant for language maps.

// PropertyContentMap is the "contentMap" property: the content in each language, keyed by BCP 47 tag.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-content
const PropertyContentMap = "contentMap"

// PropertyContext is the "context" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-context
// IMPORTANT: This is a distinct property that identifies the
// discussion or conversation context in which a post is made, and
// is NOT the same as the JSON-LD @context property.
const PropertyContext = "context"

// PropertyCurrent is the "current" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-current
const PropertyCurrent = "current"

// PropertyDeleted is the "deleted" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-deleted
const PropertyDeleted = "deleted"

// PropertyDescribes is the "describes" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-describes
const PropertyDescribes = "describes"

// PropertyDuration is the "duration" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-duration
const PropertyDuration = "duration"

// PropertyEncoding is the "encoding" property.
// https://swicg.github.io/activitypub-e2ee/mls#encoding
const PropertyEncoding = "encoding"

// PropertyEndTime is the "endTime" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-endtime
const PropertyEndTime = "endTime"

// PropertyEventStream is the "eventStream" property.
// https://swicg.github.io/activitypub-api/sse#discovery
const PropertyEventStream = "eventStream"

// PropertyFirst is the "first" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-first
const PropertyFirst = "first"

// PropertyFormerType is the "formerType" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-formertype
const PropertyFormerType = "formerType"

// PropertyGenerator is the "generator" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-generator
const PropertyGenerator = "generator"

// PropertyHeight is the "height" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-height
const PropertyHeight = "height"

// PropertyHref is the "href" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-href
const PropertyHref = "href"

// PropertyHrefLang is the "hreflang" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-hreflang
const PropertyHrefLang = "hreflang"

// PropertyIcon is the "icon" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-icon
const PropertyIcon = "icon"

// PropertyID is the "id" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-id
const PropertyID = "id"

// PropertyID_Alternate is the "@id" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-id
const PropertyID_Alternate = "@id"

// PropertyImage is the "image" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-image
const PropertyImage = "image"

// PropertyInReplyTo is the "inReplyTo" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-inreplyto
const PropertyInReplyTo = "inReplyTo"

// PropertyInstrument is the "instrument" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-instrument
const PropertyInstrument = "instrument"

// PropertyItems is the "items" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-items
const PropertyItems = "items"

// PropertyLast is the "last" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-last
const PropertyLast = "last"

// PropertyLatitude is the "latitude" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-latitude
const PropertyLatitude = "latitude"

// PropertyLocation is the "location" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-location
const PropertyLocation = "location"

// PropertyLongitude is the "longitude" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-longitude
const PropertyLongitude = "longitude"

// PropertyMediaType is the "mediaType" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-mediatype
const PropertyMediaType = "mediaType"

// PropertyMigration is the "migration" property.
// LOLA account migration extention
// https://swicg.github.io/activitypub-data-portability/lola
const PropertyMigration = "migration"

// PropertyMLSCiphersuite is the "ciphersuite" property.
// https://swicg.github.io/activitypub-e2ee/mls#ciphersuite
const PropertyMLSCiphersuite = "ciphersuite"

// PropertyName is the "name" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-name
const PropertyName = "name"

// DELTA: Emissary I18N §1.8 (ActivityPub): new constant for language maps.

// PropertyNameMap is the "nameMap" property: the name in each language, keyed by BCP 47 tag.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-name
const PropertyNameMap = "nameMap"

// PropertyNext is the "next" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-next
const PropertyNext = "next"

// PropertyObject is the "object" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-object
const PropertyObject = "object"

// PropertyOrderedItems is the "orderedItems" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-items
const PropertyOrderedItems = "orderedItems"

// PropertyOrigin is the "origin" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-origin
const PropertyOrigin = "origin"

// PropertyOneOf is the "oneOf" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-oneof
const PropertyOneOf = "oneOf"

// PropertyOwner is the "owner" property.
// https://w3c-ccg.github.io/security-vocab/#owner
const PropertyOwner = "owner"

// PropertyPartOf is the "partOf" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-partof
const PropertyPartOf = "partOf"

// PropertyPrev is the "prev" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-prev
const PropertyPrev = "prev"

// PropertyPreview is the "preview" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-preview
const PropertyPreview = "preview"

// PropertyPublished is the "published" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-published
const PropertyPublished = "published"

// PropertyRadius is the "radius" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-radius
const PropertyRadius = "radius"

// PropertyRedirectURI is the "redirectURI" property.
// Defined in FEP-d8c2: OAuth 2.0 profile for ActivityPub and
// required for Server-to-Server OAuth connections.
// https://w3id.orgp/d8c2
const PropertyRedirectURI = "redirectURI"

// PropertyRel is the "rel" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-rel
const PropertyRel = "rel"

// PropertyRelationship is the "relationship" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-relationship
const PropertyRelationship = "relationship"

// PropertyResult is the "result" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-result
const PropertyResult = "result"

// PropertyReplies is the "replies" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-replies
const PropertyReplies = "replies"

// PropertyShares is the "shares" property.
// https://www.w3.org/TR/activitypub/#shares
const PropertyShares = "shares"

// PropertyStartIndex is the "startIndex" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-startindex
const PropertyStartIndex = "startIndex"

// PropertyStartTime is the "startTime" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-starttime
const PropertyStartTime = "startTime"

// PropertySubject is the "subject" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-subject
const PropertySubject = "subject"

// PropertySummary is the "summary" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-summary
const PropertySummary = "summary"

// DELTA: Emissary I18N §1.8 (ActivityPub): new constant for language maps.

// PropertySummaryMap is the "summaryMap" property: the summary in each language, keyed by BCP 47 tag.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-summary
const PropertySummaryMap = "summaryMap"

// PropertyTag is the "tag" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-tag
const PropertyTag = "tag"

// PropertyTarget is the "target" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-target
const PropertyTarget = "target"

// PropertyTo is the "to" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-to
const PropertyTo = "to"

// PropertyTotalItems is the "totalItems" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-totalitems
const PropertyTotalItems = "totalItems"

// PropertyType is the "type" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-type
const PropertyType = "type"

// PropertyType_Alternate is the "@type" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-type
const PropertyType_Alternate = "@type"

// PropertyUnits is the "units" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-units
const PropertyUnits = "units"

// PropertyUpdated is the "updated" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-updated
const PropertyUpdated = "updated"

// PropertyURL is the "url" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-url
const PropertyURL = "url"

// PropertyWidth is the "width" property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-width
const PropertyWidth = "width"

/******************************************
 * Toot Property Types
 ******************************************/

// PropertyTootDiscoverable is the "discoverable" property.
// https://docs.joinmastodon.org/spec/activitypub/#discoverable
// http://joinmastodon.org/ns#discoverable
const PropertyTootDiscoverable = "discoverable"

// PropertyTootIndexable is the "indexable" property.
// https://docs.joinmastodon.org/spec/activitypub/#indexable
// http://joinmastodon.org/ns#indexable
const PropertyTootIndexable = "indexable"

// PropertyFeatured is the "featured" property.
// https://docs.joinmastodon.org/spec/activitypub/#featured
// http://joinmastodon.org/ns#featured
const PropertyFeatured = "featured"
