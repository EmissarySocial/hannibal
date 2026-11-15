package sigs

import (
	"crypto"
	"net/http"
	"strings"

	"github.com/benpate/derp"
	"github.com/benpate/re"
	"github.com/benpate/rosetta/list"
	"github.com/benpate/rosetta/slice"
	"github.com/rs/zerolog/log"
)

// ApplyDigest calculates the digest of the body from a given
// http.Request, then adds the digest to the Request's header.
func ApplyDigest(request *http.Request, digestName string, digestFunc DigestFunc) error {
	return applyDigest(request, re.DefaultMaximum, digestName, digestFunc)
}

// applyDigest adds a Digest header for a body of no more than maxBytes
func applyDigest(request *http.Request, maxBytes int64, digestName string, digestFunc DigestFunc) error {

	const location = "hannibal.sigs.ApplyDigest"

	if request == nil {
		return derp.Internal(location, "Request cannot be nil")
	}

	// Retrieve the request body (in a replayable manner), capped to guard against an oversized body
	body, err := re.ReadRequestBody(request, maxBytes)

	if err != nil {
		return derp.Wrap(err, location, "Unable to read request body")
	}

	if len(body) == 0 {
		return nil
	}

	// Try to calculate the digest with the DigestFunc
	result := digestName + "=" + digestFunc(body)

	// Apply the digest to the Request
	request.Header.Set(FieldDigest, result)
	return nil
}

// VerifyDigest verifies that the digest in the http.Request header
// matches the contents of the http.Request body.
func VerifyDigest(request *http.Request, allowedHashes ...crypto.Hash) error {
	return verifyDigest(request, re.DefaultMaximum, allowedHashes...)
}

// verifyDigest verifies the Digest header against a body of no more than maxBytes
func verifyDigest(request *http.Request, maxBytes int64, allowedHashes ...crypto.Hash) error {

	const location = "hannibal.sigs.VerifyDigest"

	// NILCHECK: Request cannot be nil
	if request == nil {
		return derp.Internal(location, "Request cannot be nil")
	}

	// Retrieve the request body (in a replayable manner), capped to guard against an oversized body
	body, err := re.ReadRequestBody(request, maxBytes)

	if err != nil {
		return derp.Wrap(err, location, "Unable to read request body")
	}

	// Retrieve the digest(s) included in the HTTP Request
	digestHeader := request.Header.Get(FieldDigest)

	if digestHeader == "" {

		// RULE: A body with no digest is refused, whatever the method. Nothing else ties the
		// body to the signature, so a signature made for an empty request would carry any body.
		if len(body) > 0 {
			return derp.Forbidden(location, "Request has a body, so it must include a Digest header")
		}

		// A request with no body has nothing to verify
		return nil
	}

	// Process the digest header into separate values
	headerValues := strings.Split(digestHeader, ",")
	atLeastOneAlgorithmMatches := false

	for _, headerValue := range headerValues {

		headerValue = strings.Trim(headerValue, " ")
		digestAlgorithm, digestValue := list.Split(headerValue, '=')

		// If we recognize the digest algorithm, then use it to verify the body/digest
		fn, err := getDigestFuncByName(digestAlgorithm)

		if err != nil {
			log.Trace().Msg("Hannibal sigs: VerifyDigest: Unknown digest algorithm: " + digestAlgorithm)
			continue
		}

		// Additional trace values that helped isolate a bug in the digest algorithm
		log.Trace().Msg("Validating Digest: " + digestAlgorithm + "=" + digestValue)
		log.Trace().Msg(headerValue)
		log.Trace().Msg(digestValue)
		log.Trace().Msg(fn(body))

		// If the values match, then success!
		if digestValue == fn(body) {
			log.Trace().Msg("Hannibal sigs: VerifyDigest: Valid Digest Found. Algorithm: " + digestAlgorithm)

			// Verify that this algorithm is in the list of allowed hashes
			if hash := getHashByName(digestAlgorithm); slice.Contains(allowedHashes, hash) {
				atLeastOneAlgorithmMatches = true
			}
			continue
		}

		// If the values DON'T MATCH, then fail immediately.
		// We don't want bad actors "digest shopping"
		return derp.Forbidden(location, "Digest verification failed", digestValue)
	}

	// If we have found at least one digest that matches, then success!
	if atLeastOneAlgorithmMatches {
		log.Trace().Msg("Digest verified.")
		return nil
	}

	// Otherwise, the digest hash does not meet our minimum requirements.  Fail.
	return derp.Forbidden(location, "No matching digest found")
}
