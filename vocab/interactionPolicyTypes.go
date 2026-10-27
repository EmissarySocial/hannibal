package vocab

/******************************************
 * GoToSocial Interaction Policies
 * https://docs.gotosocial.org/en/latest/federation/interaction_controls/
 ******************************************/

// PropertyInteractionPolicy is the "interactionPolicy" property, which says who may interact with an object.
// https://gotosocial.org/ns#interactionPolicy
const PropertyInteractionPolicy = "interactionPolicy"

// PropertyCanLike is the "canLike" sub-policy of an interactionPolicy.
// https://gotosocial.org/ns#canLike
const PropertyCanLike = "canLike"

// PropertyCanReply is the "canReply" sub-policy of an interactionPolicy.
// https://gotosocial.org/ns#canReply
const PropertyCanReply = "canReply"

// PropertyCanAnnounce is the "canAnnounce" sub-policy of an interactionPolicy.
// https://gotosocial.org/ns#canAnnounce
const PropertyCanAnnounce = "canAnnounce"

// PropertyCanQuote is the "canQuote" sub-policy of an interactionPolicy, defined by FEP-044f.
// https://gotosocial.org/ns#canQuote
// https://w3id.org/fep/044f
const PropertyCanQuote = "canQuote"

// PropertyAutomaticApproval lists the actors and collections whose interactions are approved at once.
// https://gotosocial.org/ns#automaticApproval
const PropertyAutomaticApproval = "automaticApproval"

// PropertyManualApproval lists the actors and collections whose interactions wait for the author's approval.
// https://gotosocial.org/ns#manualApproval
const PropertyManualApproval = "manualApproval"

// PropertyAlways is the name GoToSocial used for automaticApproval before v0.21.
// https://gotosocial.org/ns#always
const PropertyAlways = "always"

// PropertyApprovalRequired is the name GoToSocial used for manualApproval before v0.21.
// https://gotosocial.org/ns#approvalRequired
const PropertyApprovalRequired = "approvalRequired"

// PropertyInteractingObject is the interaction (a reply, a like, or a quote) that an authorization approves.
// https://gotosocial.org/ns#interactingObject
const PropertyInteractingObject = "interactingObject"

// PropertyInteractionTarget is the object being interacted with that an authorization approves.
// https://gotosocial.org/ns#interactionTarget
const PropertyInteractionTarget = "interactionTarget"

/******************************************
 * FEP-044f: Consent-respecting quote posts
 * https://w3id.org/fep/044f
 ******************************************/

// PropertyQuote is the "quote" property: the object that a quote post quotes.
// https://w3id.org/fep/044f#quote
const PropertyQuote = "quote"

// PropertyQuoteURL is the "quoteUrl" property, which FEP-044f reads for compatibility.
// https://www.w3.org/ns/activitystreams#quoteUrl
const PropertyQuoteURL = "quoteUrl"

// PropertyQuoteURI is Fedibird's "quoteUri" property, which FEP-044f reads for compatibility.
// http://fedibird.com/ns#quoteUri
const PropertyQuoteURI = "quoteUri"

// PropertyMisskeyQuote is Misskey's "_misskey_quote" property, which FEP-044f reads for compatibility.
// https://misskey-hub.net/ns#_misskey_quote
const PropertyMisskeyQuote = "_misskey_quote"

// PropertyQuoteAuthorization is the "quoteAuthorization" property: the QuoteAuthorization that approves a quote.
// https://w3id.org/fep/044f#quoteAuthorization
const PropertyQuoteAuthorization = "quoteAuthorization"

// ActivityTypeQuoteRequest is the "QuoteRequest" activity, which asks an author to approve a quote.
// https://w3id.org/fep/044f#QuoteRequest
const ActivityTypeQuoteRequest = "QuoteRequest"

// ObjectTypeQuoteAuthorization is the "QuoteAuthorization" object, which an author publishes to approve a quote.
// https://w3id.org/fep/044f#QuoteAuthorization
const ObjectTypeQuoteAuthorization = "QuoteAuthorization"
