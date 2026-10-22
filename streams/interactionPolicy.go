package streams

import (
	"encoding/json"
	"reflect"
	"slices"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
)

// InteractionPolicy is GoToSocial's "interactionPolicy" property: who may like, reply to, announce, or quote an object.
// https://docs.gotosocial.org/en/latest/federation/interaction_controls/
type InteractionPolicy struct {
	value mapof.Any
}

// NewInteractionPolicy creates an InteractionPolicy from any JSON-LD value.  A list uses its first
// item, an IRI becomes a map holding only its "id", and any other value that is not a map becomes empty.
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
	return nil // Ain't no policy like an interaction policy
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

// Map returns the InteractionPolicy's JSON-LD map, which is never nil.  The map may be shared with
// the value the policy was created from, so callers must treat it as read-only.
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
// an empty one.  A missing or null list is not stated.
func (policy InteractionPolicy) Defines(permission string) bool {

	subPolicy := interactionMap(policy.Map()[permission])

	for _, key := range []string{vocab.PropertyAutomaticApproval, vocab.PropertyManualApproval, vocab.PropertyAlways, vocab.PropertyApprovalRequired} {
		if subPolicy[key] != nil {
			return true
		}
	}

	return false
}

// AutomaticApproval returns the actors and collections whose interactions of this permission are approved at once
// https://gotosocial.org/ns#automaticApproval
func (policy InteractionPolicy) AutomaticApproval(permission string) sliceof.String {
	return policy.approvalList(permission, vocab.PropertyAutomaticApproval, vocab.PropertyAlways)
}

// ManualApproval returns the actors and collections whose interactions of this permission wait for the author's approval
// https://gotosocial.org/ns#manualApproval
func (policy InteractionPolicy) ManualApproval(permission string) sliceof.String {
	return policy.approvalList(permission, vocab.PropertyManualApproval, vocab.PropertyApprovalRequired)
}

/******************************************
 * Decision
 ******************************************/

// CanInteract returns whether the request's actor may perform the interaction named by the permission,
// such as vocab.PropertyCanReply, at once, only with the author's approval, or not at all.
func (policy InteractionPolicy) CanInteract(permission string, request InteractionRequest) InteractionResult {

	// RULE: An anonymous actor can never interact
	if request.ActorID == "" {
		return InteractionDenied
	}

	// RULE: The author may always interact with their own object
	if request.ActorID == request.AuthorID {
		return InteractionAllowed
	}

	// RULE: GoToSocial lets every mentioned actor reply
	if (permission == vocab.PropertyCanReply) && slices.Contains(request.Mentioned, request.ActorID) {
		return InteractionAllowed
	}

	// Read both lists, or the permission's default
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

	// Nobody named you, so nobody invited you
	return InteractionDenied
}

/******************************************
 * Helpers
 ******************************************/

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

	// Named slice and map types, such as MongoDB's primitive.A and primitive.M, match no case above
	default:
		if list, isList := reflectList(value); isList {
			return interactionMap(list)
		}

		if object, isObject := reflectObject(value); isObject {
			return object
		}
	}

	return mapof.NewAny()
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

// isPublicIRI returns TRUE if the IRI is any form of the ActivityStreams Public collection
func isPublicIRI(id string) bool {

	switch id {
	case vocab.NamespaceActivityStreamsPublic, vocab.NamespaceASPublic, vocab.NamespacePublic:
		return true
	}

	return false
}

// interactionIDs coerces a JSON-LD value into a list of IRIs: a string is one IRI, a map contributes
// its "id" (or JSON-LD's "@id"), and a list contributes each of its items
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

	// Named slice and map types, such as MongoDB's primitive.A and primitive.M, match no case above
	default:
		if list, isList := reflectList(value); isList {
			return interactionIDs(list)
		}

		if object, isObject := reflectObject(value); isObject {
			return interactionIDs(map[string]any(object))
		}
	}

	return result
}

// reflectList returns the items of any slice or array, whatever its named type
func reflectList(value any) ([]any, bool) {

	reflected := reflect.ValueOf(value)

	if (reflected.Kind() != reflect.Slice) && (reflected.Kind() != reflect.Array) {
		return nil, false
	}

	result := make([]any, reflected.Len())

	for index := range result {
		result[index] = reflected.Index(index).Interface()
	}

	return result, true
}

// reflectObject returns the entries of any map with string keys, whatever its named type
func reflectObject(value any) (mapof.Any, bool) {

	reflected := reflect.ValueOf(value)

	if (reflected.Kind() != reflect.Map) || (reflected.Type().Key().Kind() != reflect.String) {
		return nil, false
	}

	result := make(mapof.Any, reflected.Len())

	for iterator := reflected.MapRange(); iterator.Next(); {
		result[iterator.Key().String()] = iterator.Value().Interface()
	}

	return result, true
}
