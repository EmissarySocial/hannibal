package streams

import (
	"encoding/json"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/property"
)

/******************************************
 * Custom JSON Marshalling / Unmarshalling
 ******************************************/

// MarshalJSON implements the json.Marshaller interface,
// and provides a custom marshalling into JSON --
// essentially just aiming the marshaller at the
// Document's value.
func (document Document) MarshalJSON() ([]byte, error) {
	return json.Marshal(document.getValue().Raw())
}

// UnmarshalJSON implements the json.Unmarshaller interface,
// and provides a custom un-marshalling from JSON --
// essentially just aiming the unmashaller at the
// Document's value
func (document *Document) UnmarshalJSON(bytes []byte) error {

	const location = "hannibal.streams.Document.UnmarshalJSON"

	// Guard against a zero-value Document, whose value is a nil interface.
	// json.Unmarshal into &Document{} is the idiomatic call path, so this must
	// not panic on document.getValue().Raw().
	if document.value == nil {
		document.value = property.Nil{}
	}

	value := document.getValue().Raw()
	if err := json.Unmarshal(bytes, &value); err != nil {
		return derp.Wrap(err, location, "Error unmarshalling JSON into Document")
	}

	document.value = property.NewValue(value)
	return nil
}
