package vocab

// ContextTypeActivityStreams defines the standard ActivityStreams vocabulary.
// https://www.w3.org/TR/activitystreams-core/
const ContextTypeActivityStreams = "https://www.w3.org/ns/activitystreams"

// ContextTypeSecurity describes the standard security vocabulary for the Fediverse.
// https://w3c.github.io/vc-data-integrity/vocab/security/vocabulary.html
const ContextTypeSecurity = "https://w3id.org/security/v1"

// ContextTypeSocialWebMLS describes end-to-end-encrypted messaging using
// "Message Layer Security" (MLS) protocol defined by the Social Web Foundation
// https://swicg.github.io/activitypub-e2ee/mls
const ContextTypeSocialWebMLS = "https://purl.archive.org/socialweb/mls"

// ContextTypeToot is a context type.
// https://joinmastodon.org/ns#
var ContextTypeToot = map[string]any{
	"toot": "https://joinmastodon.org/ns#",
	// RULE: Mastodon's exact prefix, not canonical schema.org. JSON-LD readers
	// such as Funkwhale match "http://schema.org#value". See AGENTS.md.
	"schema":        "http://schema.org#",
	"PropertyValue": "schema:PropertyValue",
	"value":         "schema:value",
	"discoverable":  "toot:discoverable",
	"indexable":     "toot:indexable",
	"featured": map[string]any{
		"@id":   "http://joinmastodon.org/ns#featured",
		"@type": "@id",
	},
}

// ContextTypeGoToSocial declares GoToSocial's interaction policy terms.
// https://docs.gotosocial.org/en/latest/federation/interaction_controls/
var ContextTypeGoToSocial = map[string]any{
	"gts":                     "https://gotosocial.org/ns#",
	PropertyInteractionPolicy: map[string]any{"@id": "gts:interactionPolicy", "@type": "@id"},
	PropertyCanLike:           map[string]any{"@id": "gts:canLike", "@type": "@id"},
	PropertyCanReply:          map[string]any{"@id": "gts:canReply", "@type": "@id"},
	PropertyCanAnnounce:       map[string]any{"@id": "gts:canAnnounce", "@type": "@id"},
	PropertyCanQuote:          map[string]any{"@id": "gts:canQuote", "@type": "@id"},
	PropertyAutomaticApproval: map[string]any{"@id": "gts:automaticApproval", "@type": "@id"},
	PropertyManualApproval:    map[string]any{"@id": "gts:manualApproval", "@type": "@id"},
	PropertyInteractingObject: map[string]any{"@id": "gts:interactingObject", "@type": "@id"},
	PropertyInteractionTarget: map[string]any{"@id": "gts:interactionTarget", "@type": "@id"},
}

// ContextTypeQuotePosts declares the terms of FEP-044f: Consent-respecting quote posts.
// https://w3id.org/fep/044f
var ContextTypeQuotePosts = map[string]any{
	PropertyQuote:                map[string]any{"@id": "https://w3id.org/fep/044f#quote", "@type": "@id"},
	PropertyQuoteAuthorization:   map[string]any{"@id": "https://w3id.org/fep/044f#quoteAuthorization", "@type": "@id"},
	ActivityTypeQuoteRequest:     "https://w3id.org/fep/044f#QuoteRequest",
	ObjectTypeQuoteAuthorization: "https://w3id.org/fep/044f#QuoteAuthorization",
}
