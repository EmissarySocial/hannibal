package validator

import (
	"net/http"

	"github.com/benpate/hannibal/streams"
)

// None is a Validator that accepts every activity without checking it, for routes that
// authenticate requests some other way, such as with a cookie or an API key
type None struct{}

// NewNone returns a Validator that accepts every activity without checking it.
func NewNone() None {
	return None{}
}

// Validate implements the Validator interface, and always returns ResultValid.
func (validator None) Validate(_ *http.Request, _ *streams.Document) Result {
	return ResultValid
}
