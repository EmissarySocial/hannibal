package validator

import (
	"net/http"

	"github.com/benpate/hannibal/streams"
)

// None is a Validator that always returns VALID because
// the request has already been validated in some external way.
// For example, the route may require a Cookie or API Key for access
type None struct{}

// NewNone returns a Validator that performs no validation and always returns ResultUnknown.
func NewNone() None {
	return None{}
}

// Validate uses the hannibal/sigs library to verify that the HTTP
// request is signed with a valid key.
func (validator None) Validate(_ *http.Request, _ *streams.Document) Result {
	return ResultValid
}
