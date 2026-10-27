package validator

import (
	"net/http"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
)

// HTTPLookup is a Validator that tries to retrieve the original document from the source server
type HTTPLookup struct{}

// NewHTTPLookup returns a Validator that confirms an activity exists by fetching it from its origin server.
func NewHTTPLookup() HTTPLookup {
	return HTTPLookup{}
}

// Validate confirms the activity exists by fetching it from its origin server.
func (v HTTPLookup) Validate(_ *http.Request, activity *streams.Document) Result {

	const location = "hannibal.validator.HTTPLookup"

	switch activity.Type() {
	case vocab.ActivityTypeCreate, vocab.ActivityTypeUpdate:
	default:
		return ResultUnknown
	}

	// Get the ObjectID of the document
	object, err := activity.Object().Load()

	if err != nil {
		derp.Report(derp.Wrap(err, location, "Loading original document"))
	}

	// Extract the value from the "original" retrieved document and replace it int the
	// document that was passed in
	propertyValue := property.NewValue(object.Value())
	activity.SetValue(propertyValue)

	// Return in triumph
	return ResultValid
}
