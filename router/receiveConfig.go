package router

import (
	"github.com/benpate/hannibal/validator"
	"github.com/benpate/re"
)

// ReceiveConfig is a configuration object for the `ReceiveRequest` function.
type ReceiveConfig struct {
	Validators  []Validator
	MaxBodySize int64 // Maximum number of bytes to read from an inbound request body. Zero uses re.DefaultMaximum.
}

// NewReceiveConfig creates a new ReceiveConfig object with default settings,
// and applies any provided options to override the defaults.
func NewReceiveConfig(options ...Option) ReceiveConfig {

	result := ReceiveConfig{
		MaxBodySize: re.DefaultMaximum,
		Validators: []Validator{

			// checks HTTP signatures (nil = use default key finder)
			validator.NewHTTPSig(nil),

			// checks if objects have been deleted
			validator.NewDeletedObject(),
		},
	}

	for _, option := range options {
		option(&result)
	}

	return result
}
