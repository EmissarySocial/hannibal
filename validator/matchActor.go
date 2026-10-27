package validator

import (
	"net/http"

	"github.com/benpate/hannibal/streams"
)

// MatchActor is a Validator that verifies that the activity
// actor value matches an already authenticated Actor URL
type MatchActor struct {
	actorID string
}

// NewMatchActor returns a Validator that confirms the activity's actor matches the expected actor ID.
func NewMatchActor(actorID string) MatchActor {
	return MatchActor{
		actorID: actorID,
	}
}

// Validate uses the hannibal/sigs library to verify that the HTTP
// request is signed by a specific actor.
func (validator MatchActor) Validate(_ *http.Request, activity *streams.Document) Result {

	if activity.Actor().ID() == validator.actorID {
		return ResultValid
	}
	return ResultInvalid
}
