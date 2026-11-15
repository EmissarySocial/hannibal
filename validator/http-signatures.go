package validator

import (
	"net/http"
	"slices"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/hannibal/streams"
	"github.com/rs/zerolog/log"
)

// HTTPSig is a Validator that checks incoming HTTP requests
// using the HTTP signatures algorithm.
// https://docs.joinmastodon.org/spec/security/
type HTTPSig struct {
	keyFinder sigs.PublicKeyFinder
	options   []sigs.VerifierOption
}

// NewHTTPSig returns a fully initialized HTTPSig validator. The provided
// keyFinder is OPTIONAL: if it is nil, the validator uses its default behavior
// of loading the signing Actor's public key from the inbound document. Any
// options are passed through to sigs.Verify on every request.
func NewHTTPSig(keyFinder sigs.PublicKeyFinder, options ...sigs.VerifierOption) HTTPSig {

	return HTTPSig{
		keyFinder: keyFinder,
		options:   options,
	}
}

// WithKeyFinder returns a copy of this validator that uses another key finder, and keeps its options
func (validator HTTPSig) WithKeyFinder(keyFinder sigs.PublicKeyFinder) HTTPSig {
	validator.keyFinder = keyFinder
	return validator
}

// WithOptions returns a copy of this validator with more verifier options, applied after its own
func (validator HTTPSig) WithOptions(options ...sigs.VerifierOption) HTTPSig {
	validator.options = append(slices.Clone(validator.options), options...)
	return validator
}

// Validate uses the hannibal/sigs library to verify that the HTTP
// request is signed with a valid key.
func (validator HTTPSig) Validate(request *http.Request, activity *streams.Document) Result {

	if !sigs.HasSignature(request) {
		return ResultUnknown
	}

	// Try to use the KeyFinder configured in this Validator.
	keyFinder := validator.keyFinder

	// If none is provided, then use the default KeyFinder, which looks up the Actor's public key from the document.
	if keyFinder == nil {
		keyFinder = defaultKeyFinder(activity)
	}

	// Note whether the key could be found, which is not the same as a signature that is wrong
	keyNotFound := false

	findKey := func(keyID string) (string, error) {

		key, err := keyFinder(keyID)

		if err != nil {
			keyNotFound = true
		}

		return key, err
	}

	// Verify the request using the Actor's public key
	signature, err := sigs.Verify(request, findKey, validator.options...)

	// RULE: A signature whose key cannot be found proves nothing either way, so the next
	// validator decides. A deleted account signs its last Delete with a key that is already gone.
	if (err != nil) && keyNotFound {
		log.Trace().Err(err).Msg("Hannibal Inbox: Signing key could not be found")
		return ResultUnknown
	}

	if err != nil {
		log.Trace().Err(err).Msg("Hannibal Inbox: Error verifying HTTP Signature")
		return ResultInvalid
	}

	// Actor who owns the signature must match the Actor in the Activity.
	if signature.ActorID() != activity.Actor().ID() {
		log.Trace().Str("signatureActor", signature.ActorID()).Str("activityActor", activity.Actor().ID()).Msg("Hannibal Inbox: HTTP Signature Actor does not match Activity Actor")
		return ResultInvalid
	}

	log.Trace().Msg("Hannibal Inbox: HTTP Signature Verified")
	return ResultValid
}

// defaultKeyFinder looks up the public key of the activity's actor. It loads the actor with a
// new default client, and not with the client that the activity carries.
func defaultKeyFinder(activity *streams.Document) sigs.PublicKeyFinder {

	const location = "hannibal.validator.defaultKeyFinder"

	return func(keyID string) (string, error) {

		// Create a fresh client to load the Actor from the activity
		actor, err := streams.NewDocument(activity.Actor().ID(), streams.WithDefaultClient()).Load()

		if err != nil {
			return "", derp.Wrap(err, location, "Retrieving Actor from ActivityPub activity", activity.Value())
		}

		// Search the Actor's public keys for the one that matches the provided keyID
		for key := actor.PublicKey(); key.NotNil(); key = key.Tail() {

			// Verify that the key ID retrieved from the Actor matches the key ID provided in the Signature
			// Without this step, it is possible for an attacker to sign a request with a key that does not belong to the Actor.
			if key.ID() == keyID {
				return key.PublicKeyPEM(), nil
			}
		}

		// If none match, then return a (hopefully informative) error.
		log.Trace().Str("keyId", keyID).Msg("Hannibal Inbox: Could not find remote actor's public key")
		return "", derp.BadRequest(location, "Actor must publish the key used to sign this request", actor.ID(), keyID)
	}
}
