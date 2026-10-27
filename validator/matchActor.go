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

// Validate implements the Validator interface. It returns ResultValid when the activity's
// actor is the expected actor, and ResultInvalid otherwise.
func (validator MatchActor) Validate(_ *http.Request, activity *streams.Document) Result {

	if activity.Actor().ID() == validator.actorID {
		return ResultValid
	}
	return ResultInvalid
}
