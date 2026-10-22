package streams

import (
	"encoding/json"
	"slices"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
)

// InteractionPolicy represents GoToSocial's "interactionPolicy" property, which says who may like,
// reply to, announce, or quote an object.  FEP-044f adds the canQuote sub-policy.
// https://docs.gotosocial.org/en/latest/federation/interaction_controls/
// https://w3id.org/fep/044f
type InteractionPolicy struct {
	value mapof.Any
}

// InteractionResult is the answer an InteractionPolicy gives for one interaction
type InteractionResult int

const (
	// InteractionDenied means the interaction is not approved.  It is the zero value, so an
	// unset result never allows anything.
	InteractionDenied InteractionResult = iota

	// InteractionNeedsApproval means the interaction waits for the author's approval
	InteractionNeedsApproval

	// InteractionAllowed means the interaction is approved at once
	InteractionAllowed
)

// InteractionRequest describes the actor asking to interact with an object, and the object's
// own actors, which a policy cannot know by itself
type InteractionRequest struct {

	// ActorID is the actor who wants to interact
	ActorID string

	// AuthorID is the object's author, who may always interact with it
	AuthorID string

	// Mentioned lists the actors the object mentions or replies to, who may always reply to it
	Mentioned []string

	// IsMember returns TRUE if the actor belongs to a collection named in the policy, such as the
	// author's followers.  It is called only for IRIs that are neither Public nor an actor,
	// because each answer may cost the caller a lookup.
	IsMember func(collectionID string) bool
}

// NewInteractionPolicy creates a new InteractionPolicy from a JSON-LD value.  A map is used as-is,
// a list uses its first item, and an IRI becomes a map holding only its "id".  Any other value
// becomes an empty policy, which allows everything that GoToSocial allows by default.
func NewInteractionPolicy(value any) InteractionPolicy {
	return InteractionPolicy{value: interactionMap(value)}
}

// UnmarshalJSON decodes any JSON value into an InteractionPolicy, using the same rules as NewInteractionPolicy
func (policy *InteractionPolicy) UnmarshalJSON(data []byte) error {

	const location = "hannibal.streams.InteractionPolicy.UnmarshalJSON"

	var value any

	if err := json.Unmarshal(data, &value); err != nil {
		return derp.Wrap(err, location, "Unable to decode InteractionPolicy", derp.WithBadRequest())
	}

	*policy = NewInteractionPolicy(value)
	return nil
}

// MarshalJSON encodes the InteractionPolicy as its JSON-LD map
func (policy InteractionPolicy) MarshalJSON() ([]byte, error) {

	const location = "hannibal.streams.InteractionPolicy.MarshalJSON"

	result, err := json.Marshal(policy.Map())

	if err != nil {
		return nil, derp.Wrap(err, location, "Unable to encode InteractionPolicy")
	}

	return result, nil
}

/******************************************
 * Accessors
 ******************************************/

// Map returns the InteractionPolicy's JSON-LD map, which is never nil
func (policy InteractionPolicy) Map() mapof.Any {

	if policy.value == nil {
		return mapof.NewAny()
	}

	return policy.value
}

// ID returns the InteractionPolicy's ID, which is set only when the policy was published as an IRI
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-id
func (policy InteractionPolicy) ID() string {
	return policy.Map().GetString(vocab.PropertyID)
}

// IsNil returns TRUE if the InteractionPolicy states no sub-policies
func (policy InteractionPolicy) IsNil() bool {

	for _, permission := range []string{vocab.PropertyCanLike, vocab.PropertyCanReply, vocab.PropertyCanAnnounce, vocab.PropertyCanQuote} {
		if policy.Defines(permission) {
			return false
		}
	}

	return true
}

// NotNil returns TRUE if the InteractionPolicy states at least one sub-policy
func (policy InteractionPolicy) NotNil() bool {
	return !policy.IsNil()
}

// Defines returns TRUE if the InteractionPolicy states either approval list for the permission, even
// an empty one.  A null list is not stated, and a permission it does not define uses its default.
// https://docs.gotosocial.org/en/latest/federation/interaction_controls/
func (policy InteractionPolicy) Defines(permission string) bool {

	subPolicy := interactionMap(policy.Map()[permission])

	for _, key := range []string{vocab.PropertyAutomaticApproval, vocab.PropertyManualApproval, vocab.PropertyAlways, vocab.PropertyApprovalRequired} {
		if subPolicy[key] != nil {
			return true
		}
	}

	return false
}

// AutomaticApproval returns the actors and collections whose interactions of this permission are
// approved at once.  It reads GoToSocial's pre-v0.21 "always" when "automaticApproval" is missing.
// https://gotosocial.org/ns#automaticApproval
func (policy InteractionPolicy) AutomaticApproval(permission string) sliceof.String {
	return policy.approvalList(permission, vocab.PropertyAutomaticApproval, vocab.PropertyAlways)
}

// ManualApproval returns the actors and collections whose interactions of this permission wait for
// the author's approval.  It reads GoToSocial's pre-v0.21 "approvalRequired" when "manualApproval" is missing.
// https://gotosocial.org/ns#manualApproval
func (policy InteractionPolicy) ManualApproval(permission string) sliceof.String {
	return policy.approvalList(permission, vocab.PropertyManualApproval, vocab.PropertyApprovalRequired)
}

/******************************************
 * Decision
 ******************************************/

// CanInteract returns whether the request's actor may perform this interaction (vocab.PropertyCanLike,
// PropertyCanReply, PropertyCanAnnounce, or PropertyCanQuote) at once, only with the author's
// approval, or not at all.  It applies GoToSocial's rules:
//
//   - The author may always interact with their own object.  Under FEP-044f, a self-quote is the only
//     quote that needs no approval.
//   - An actor the object mentions or replies to may always reply.
//   - A permission the policy does not define approves Public at once, except canQuote, which
//     approves no one, because FEP-044f requires approval for every quote but a self-quote.
//   - The most specific entry wins: an actor's own IRI, then a collection, then Public.  At the
//     same level, automatic approval wins over manual approval.
//
// GoToSocial's exception for an object that is itself pending approval is not applied, because a
// policy cannot know whether its object is pending.
func (policy InteractionPolicy) CanInteract(permission string, request InteractionRequest) InteractionResult {

	// RULE: An anonymous actor can never interact
	if request.ActorID == "" {
		return InteractionDenied
	}

	if request.ActorID == request.AuthorID {
		return InteractionAllowed
	}

	if (permission == vocab.PropertyCanReply) && slices.Contains(request.Mentioned, request.ActorID) {
		return InteractionAllowed
	}

	automatic, manual := policy.approvalLists(permission)

	// The actor's own IRI is the most specific entry
	if automatic.Contains(request.ActorID) {
		return InteractionAllowed
	}

	if manual.Contains(request.ActorID) {
		return InteractionNeedsApproval
	}

	// A collection is more specific than Public
	if request.isMemberOfAny(automatic) {
		return InteractionAllowed
	}

	if request.isMemberOfAny(manual) {
		return InteractionNeedsApproval
	}

	// Public is the least specific entry
	if slices.ContainsFunc(automatic, isPublicIRI) {
		return InteractionAllowed
	}

	if slices.ContainsFunc(manual, isPublicIRI) {
		return InteractionNeedsApproval
	}

	return InteractionDenied
}

/******************************************
 * Helpers
 ******************************************/

// approvalLists returns the automatic and manual approval lists for a permission, with that
// permission's default when the policy does not define it
func (policy InteractionPolicy) approvalLists(permission string) (sliceof.String, sliceof.String) {

	if policy.Defines(permission) {
		return policy.AutomaticApproval(permission), policy.ManualApproval(permission)
	}

	// RULE: FEP-044f requires approval for every quote but a self-quote
	if permission == vocab.PropertyCanQuote {
		return sliceof.NewString(), sliceof.NewString()
	}

	// RULE: GoToSocial approves Public at once for any permission a policy does not define
	return sliceof.String{vocab.NamespaceActivityStreamsPublic}, sliceof.NewString()
}

// approvalList returns the IRIs in one approval list of a sub-policy, reading the legacy key only
// when the current key is missing or null
func (policy InteractionPolicy) approvalList(permission string, key string, legacyKey string) sliceof.String {

	subPolicy := interactionMap(policy.Map()[permission])

	// RULE: JSON null means "unset", so it never hides the legacy key
	if value := subPolicy[key]; value != nil {
		return interactionIDs(value)
	}

	return interactionIDs(subPolicy[legacyKey])
}

// isMemberOfAny returns TRUE if the request's actor belongs to any collection in the list.  It skips
// Public and the object's own actors, which are actors and not collections.
func (request InteractionRequest) isMemberOfAny(ids sliceof.String) bool {

	if request.IsMember == nil {
		return false
	}

	for _, id := range ids {

		if isPublicIRI(id) || (id == request.ActorID) || (id == request.AuthorID) {
			continue
		}

		if request.IsMember(id) {
			return true
		}
	}

	return false
}

// isPublicIRI returns TRUE if the IRI is any form of the ActivityStreams Public collection
func isPublicIRI(id string) bool {

	switch id {
	case vocab.NamespaceActivityStreamsPublic, vocab.NamespaceASPublic, vocab.NamespacePublic:
		return true
	}

	return false
}

// interactionMap coerces a JSON-LD value into a map: a map is used as-is, a list uses its first
// item, an IRI becomes a map holding only its "id", and any other value becomes an empty map
func interactionMap(value any) mapof.Any {

	switch typed := value.(type) {

	case InteractionPolicy:
		return typed.Map()

	case Document:
		return interactionMap(typed.Value())

	case property.Value:
		return interactionMap(typed.Raw())

	case mapof.Any:
		return typed

	case map[string]any:
		return mapof.Any(typed)

	case sliceof.Any:
		return interactionMap([]any(typed))

	case []any:
		if len(typed) > 0 {
			return interactionMap(typed[0])
		}

	case string:
		if typed != "" {
			return mapof.Any{vocab.PropertyID: typed}
		}
	}

	return mapof.NewAny()
}

// interactionIDs coerces a JSON-LD value into a list of IRIs: a string is one IRI, a map
// contributes its "id" (or JSON-LD's "@id"), and a list contributes each of its items
func interactionIDs(value any) sliceof.String {

	result := sliceof.NewString()

	switch typed := value.(type) {

	case Document:
		return interactionIDs(typed.Value())

	case property.Value:
		return interactionIDs(typed.Raw())

	case string:
		if typed != "" {
			result = append(result, typed)
		}

	case mapof.Any:
		return interactionIDs(map[string]any(typed))

	case map[string]any:

		// Read the ActivityStreams "id" first, and the JSON-LD "@id" second, as Document.ID does
		if id := interactionIDs(typed[vocab.PropertyID]); id.NotEmpty() {
			return id
		}

		return interactionIDs(typed[vocab.PropertyID_Alternate])

	case []string:
		for _, item := range typed {
			result = append(result, interactionIDs(item)...)
		}

	case sliceof.String:
		return interactionIDs([]string(typed))

	case sliceof.Any:
		return interactionIDs([]any(typed))

	case []any:
		for _, item := range typed {
			result = append(result, interactionIDs(item)...)
		}
	}

	return result
}
