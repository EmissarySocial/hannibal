package router

import (
	"slices"

	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/hannibal/validator"
	"github.com/benpate/re"
)

// ReceiveConfig is a configuration object for the `ReceiveRequest` function.
type ReceiveConfig struct {
	Validators  []Validator
	MaxBodySize int64 // Maximum number of bytes to read from an inbound request body. Zero uses re.DefaultMaximum.

	keyFinder sigs.PublicKeyFinder // If present, replaces the key finder of every HTTPSig validator
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

	result.configureSignatureValidators()
	return result
}

// configureSignatureValidators gives every HTTPSig validator the settings that belong to the
// whole request. It runs after every option, so that no option depends on its place in the list.
func (config *ReceiveConfig) configureSignatureValidators() {

	// The caller's slice may be shared, so the changed validators go into a copy
	config.Validators = slices.Clone(config.Validators)

	for index, item := range config.Validators {

		signatureValidator, isSignatureValidator := item.(validator.HTTPSig)

		if !isSignatureValidator {
			continue
		}

		if config.keyFinder != nil {
			signatureValidator = signatureValidator.WithKeyFinder(config.keyFinder)
		}

		// RULE: The Digest check reads the same body that ReceiveRequest read, under the same limit
		config.Validators[index] = signatureValidator.WithOptions(sigs.VerifierMaxBodySize(config.MaxBodySize))
	}
}
