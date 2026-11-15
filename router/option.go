package router

import (
	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/re"
)

// Option is a function that configures a ReceiveConfig.
type Option func(*ReceiveConfig)

// WithValidators replaces the validator chain used to verify inbound activities.
func WithValidators(validators ...Validator) Option {
	return func(config *ReceiveConfig) {
		config.Validators = validators
	}
}

// WithMaxBodySize sets the maximum number of bytes that ReceiveRequest will read
// from an inbound request body. A value of zero (or less) restores the default.
func WithMaxBodySize(maxBytes int64) Option {
	return func(config *ReceiveConfig) {
		if maxBytes <= 0 {
			maxBytes = re.DefaultMaximum
		}
		config.MaxBodySize = maxBytes
	}
}

// WithPublicKeyFinder configures the HTTP signature validator to use the
// provided public key finder when verifying inbound requests. This replaces
// the key finder of every HTTPSig validator in the chain, and keeps each one's verifier
// options. It applies after every other option, wherever it appears in the list.
func WithPublicKeyFinder(keyFinder sigs.PublicKeyFinder) Option {
	return func(config *ReceiveConfig) {
		config.keyFinder = keyFinder
	}
}
