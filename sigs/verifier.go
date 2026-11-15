package sigs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/re"
	"github.com/benpate/rosetta/slice"
	"github.com/rs/zerolog/log"
)

// Verifier contains all of the settings necessary to verify a request
type Verifier struct {
	Fields          []string
	BodyDigests     []crypto.Hash   // List of algorithms to accept from remote servers when they create a Digest header.  Default is SHA256 and SHA512
	SignatureHashes []crypto.Hash   // Digest algorithm used to create the signature.  Default is SHA256, SHA512
	Timeout         int             // Number of seconds before signatures are expired. Default is 43200 seconds (12 hours).
	CheckDigest     bool            // If true, then the verifier will check the Digest header.  Default is true.
	RefreshKey      PublicKeyFinder // If present, this is consulted when a signature fails, to detect a rotated key.  Default is nil.
	MaxBodySize     int64           // Largest body that the Digest check reads. Default is re.DefaultMaximum.
	Hostname        string          // If set, the host that requests are signed for. Default is the request's own Host.
}

// NewVerifier returns a fully initialized Verifier
func NewVerifier(options ...VerifierOption) Verifier {
	result := Verifier{
		Fields:          []string{FieldRequestTarget, FieldHost, FieldDate, FieldDigest},
		BodyDigests:     []crypto.Hash{crypto.SHA256, crypto.SHA512},
		SignatureHashes: []crypto.Hash{crypto.SHA256, crypto.SHA512},
		Timeout:         12 * 60 * 60, // 12 hours
		CheckDigest:     true,
		MaxBodySize:     re.DefaultMaximum,
	}
	result.Use(options...)
	return result
}

// Verify verifies the given http.Request. This is
// syntactic sugar for NewVerifier(options...).Verify(request)
func Verify(request *http.Request, keyFinder PublicKeyFinder, options ...VerifierOption) (Signature, error) {

	const location = "hannibal.sigs.Verify"

	// RULE: Request cannot be nil
	if request == nil {
		return Signature{}, derp.Internal(location, "Request cannot be nil")
	}

	verifier := NewVerifier()

	// If this is a GET request, then the default should not require the body digest.
	if request.Method == "GET" {
		verifier.Fields = []string{FieldRequestTarget, FieldHost, FieldDate}
	}

	// Use all other options
	verifier.Use(options...)

	// This verifier is hot-to-go.
	return verifier.Verify(request, keyFinder)
}

// Use applies the given options to the Verifier
func (verifier *Verifier) Use(options ...VerifierOption) {
	for _, option := range options {
		option(verifier)
	}
}

// Verify verifies the given http.Request
func (verifier *Verifier) Verify(request *http.Request, keyFinder PublicKeyFinder) (Signature, error) {

	const location = "hannibal.sigs.Verify"

	if request == nil {
		return Signature{}, derp.Internal(location, "Request cannot be nil")
	}

	log.Trace().
		Str("loc", location).
		Msg("Verifying Signature")

	// Verify the request date
	if verifier.Timeout > 0 {

		dateString := request.Header.Get(FieldDate)

		// RULE: A verifier that requires the date in the signature requires the header too.
		// Otherwise a signature made over an empty date would never expire.
		if (dateString == "") && slice.Contains(verifier.Fields, FieldDate) {
			return Signature{}, derp.Forbidden(location, "Request must include a Date header")
		}

		// An empty date is allowed only when the verifier does not require the sender to sign it
		if dateString != "" {

			date, err := parseDateHeader(dateString)

			if err != nil {
				return Signature{}, derp.Wrap(err, location, "Invalid Date header.  Must match 'Mon, 02 Jan 2006 15:04:05 GMT'")
			}

			window := time.Duration(verifier.Timeout) * time.Second
			now := time.Now()

			if date.Before(now.Add(-window)) {
				return Signature{}, derp.Forbidden(location, "Request date has expired. Must be within the last "+strconv.Itoa(verifier.Timeout)+" seconds")
			}

			// RULE: A date in the future is refused too, or a sender could make a signature that
			// stays valid for as long as it likes
			if date.After(now.Add(window)) {
				return Signature{}, derp.Forbidden(location, "Request date is in the future. Must be within the next "+strconv.Itoa(verifier.Timeout)+" seconds")
			}
		}
	}

	// Verify the body Digest (default behavior)
	if verifier.CheckDigest {
		if err := verifyDigest(request, verifier.maxBodySize(), verifier.BodyDigests...); err != nil {
			return Signature{}, derp.Wrap(err, location, "Error verifying body digest")
		}
	}

	// Retrieve and parse the Signature from the HTTP Request
	signature, err := ParseSignature(GetSignature(request))

	if err != nil {
		return Signature{}, derp.Wrap(err, location, "Error parsing signature")
	}

	// RULE: If the signature has expired, then reject it.
	if signature.IsExpired(verifier.Timeout) {
		return signature, derp.Forbidden(location, "Signature has expired")
	}

	// RULE: Verify that the signature contains all of the fields that we require
	if !slice.ContainsAll(signature.Headers, verifier.Fields...) {
		return signature, derp.Forbidden(location, "Signature must include ALL of these fields", verifier.Fields)
	}

	// Retrieve the public key used for this Signature
	certificate, err := keyFinder(signature.KeyID)

	if err != nil {
		return signature, derp.Wrap(err, location, "Error retrieving public signing key", signature.KeyID)
	}

	log.Trace().
		Str("loc", location).
		Str("certificate", certificate).
		Interface("signature", signature).
		Msg("Hannibal sigs: Parsed Signature")

	// Verify the signature against the key we were given
	err = verifier.verifyWithKey(request, signature, certificate)

	if err == nil {
		return signature, nil
	}

	// RULE: Without a RefreshKey function, a failed verification is final.
	if verifier.RefreshKey == nil {
		return signature, err
	}

	// A failure here MAY mean that the remote server has rotated its key and the caller is holding a
	// stale copy.  Everything that could fail for another reason -- a bad date, a bad digest, an
	// unparseable signature, an unreachable key -- has already returned above, so reaching this point
	// is what makes the extra lookup worth its cost.
	refreshed, refreshErr := verifier.RefreshKey(signature.KeyID)

	// RULE: A refresh that fails tells us nothing that the first attempt did not, so the original
	// error stands.  The refresh error is dropped on purpose: for a forged keyID this path fails as a
	// matter of course, and reporting it would let a sender fill the caller's log.
	if refreshErr != nil {
		return signature, err
	}

	// RULE: An unchanged key means the signature is simply bad.  Repeating the same check against the
	// same key would only produce the same answer, one crypto operation later.
	if refreshed == certificate {
		return signature, err
	}

	// The remote HAS rotated.  Verify once more, against the key they are using now.
	if err := verifier.verifyWithKey(request, signature, refreshed); err != nil {
		return signature, derp.Wrap(err, location, "Signature is invalid, including against the refreshed key", signature.KeyID)
	}

	return signature, nil
}

// maxBodySize returns the largest body that the Digest check reads
func (verifier *Verifier) maxBodySize() int64 {

	if verifier.MaxBodySize <= 0 {
		return re.DefaultMaximum
	}

	return verifier.MaxBodySize
}

// verifyWithKey decodes a single PEM certificate and tries the Signature against each of the
// Verifier's hash algorithms, returning nil if any one of them matches.
func (verifier *Verifier) verifyWithKey(request *http.Request, signature Signature, certificate string) error {

	const location = "hannibal.sigs.Verify"

	// Decode the PEM certificate into a public key
	publicKey, err := DecodePublicPEM(certificate)

	if err != nil {
		return derp.Wrap(err, location, "Unable to decode public key", certificate)
	}

	log.Trace().
		Str("loc", location).
		Str("pem", certificate).
		Msg("Hannibal sigs: Decoded Public Key")

	// Recreate the plaintext and digest used to make the Signature.  This reads HEADERS only, so it is
	// safe to call a second time -- unlike the body Digest, which is checked once, further up.
	plaintext := makePlaintext(request, signature, verifier.Hostname, signature.Headers...)

	// Try each hash in order
	for _, hash := range verifier.SignatureHashes {
		if err := verifyHashAndSignature(plaintext, hash, publicKey, signature.Signature); err == nil {
			log.Trace().Str("loc", location).Msg("Hannibal.sigs: Found valid signature")
			return nil
		} else if canTrace() {
			log.Trace().Msg(".......")
			log.Trace().Str("loc", location).Str("hash", hash.String()).Err(err).Msg("Hannibal.sigs: Error validating signature")
			derp.Report(derp.Wrap(err, location, "Unable to validate signature", plaintext, hash, certificate, signature))
		}
	}

	return derp.Forbidden(location, "No valid signatures found")
}

/******************************************
 * Helper Functions
 ******************************************/

// Verify Hash And Signature computes the hashed value of the plaintext, then verifies
// that this result matches the provided public key and signature.  It returns an error
// if the signature does not match.
func verifyHashAndSignature(plaintext string, hash crypto.Hash, publicKey crypto.PublicKey, signature []byte) error {

	const location = "hannibal.sigs.verifyHashAndSignature"

	// Make a digest using the hash algorithm
	digest, err := makeSignatureHash(plaintext, hash)

	if err != nil {
		return derp.Wrap(err, location, "Unable to create digest")
	}

	// Logging here.. wrapping it in an "if" because the base64 encoding is "expensive"
	if canTrace() {
		log.Trace().
			Str("plaintext", plaintext).
			Int("hash", int(hash)).
			Str("signature", base64.StdEncoding.EncodeToString(signature)).
			Str("digest", base64.StdEncoding.EncodeToString(digest)).
			Msg("Hannibal sigs: Digest Created")
	}

	// Verify the signature matches the message digest
	if err := verifySignature(publicKey, hash, digest, signature); err != nil {
		err = derp.Wrap(err, location, "Signature is invalid")

		// A signature that fails one hash is routine -- the caller tries the next one, and only
		// gives up after all of them -- so this is a trace aid, never a production error report.
		if canTrace() {
			derp.Report(err)
		}

		return err
	}

	// Beauty is in the eye of the beholder.
	return nil
}

// verifySignature verifies the given signature using the provided public key.
// The public key can be either an RSA or ECDSA keys.
func verifySignature(publicKey crypto.PublicKey, hash crypto.Hash, digest []byte, signature []byte) error {

	const location = "hannibal.sigs.verifySignature"

	switch typedKey := publicKey.(type) {

	case *rsa.PublicKey:
		if err := rsa.VerifyPKCS1v15(typedKey, hash, digest, signature); err != nil {
			return derp.Wrap(err, location, "Error verifying RSA signature")
		}
		return nil

	case *ecdsa.PublicKey:
		if !ecdsa.VerifyASN1(typedKey, digest, signature) {
			return derp.Forbidden(location, "Invalid ECDSA signature")
		}
		return nil
	}

	return derp.BadRequest(location, "Unrecognized public key type")
}
