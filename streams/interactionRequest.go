package streams

import "github.com/benpate/rosetta/sliceof"

// InteractionRequest describes the actor asking to interact with an object, and the object's own
// actors, which an InteractionPolicy cannot know by itself
type InteractionRequest struct {

	// ActorID is the actor who wants to interact
	ActorID string

	// AuthorID is the object's author, who may always interact with it
	AuthorID string

	// Mentioned lists the actors the object mentions, who may always reply to it
	Mentioned []string

	// IsMember returns TRUE if the actor belongs to a collection, such as the author's followers
	IsMember func(collectionID string) bool
}

// isMemberOfAny returns TRUE if the request's actor belongs to any collection in the list.  It skips
// Public and the object's own actors, which are actors and not collections.
func (request InteractionRequest) isMemberOfAny(ids sliceof.String) bool {

	// RULE: Without IsMember, the actor belongs to no collection
	if request.IsMember == nil {
		return false
	}

	for _, id := range ids {

		// RULE: Ask only about IRIs that might be collections, because each answer costs the caller a lookup
		if isPublicIRI(id) || (id == request.ActorID) || (id == request.AuthorID) {
			continue
		}

		if request.IsMember(id) {
			return true
		}
	}

	return false
}
