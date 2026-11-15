package sigs

import "crypto"

// VerifierOption is a function that modifies a Verifier
type VerifierOption func(*Verifier)

// VerifierFields sets the list of http.Request fields that
// MUST ALL be present in the "Signature" header from a
// remote server for a signature to be accepted. Extra
// fields are allowed in the Signature, and will still
// be verified.
func VerifierFields(fields ...string) VerifierOption {
	return func(verifier *Verifier) {
		verifier.Fields = fields
	}
}

// VerifierBodyDigests sets the list of algorithms accepted from remote servers when
// they create a "Digest" header. ALL recognized digests must be valid to pass, and AT
// LEAST ONE of the algorithms must be from this list.
func VerifierBodyDigests(digests ...crypto.Hash) VerifierOption {
	return func(verifier *Verifier) {
		verifier.BodyDigests = digests
	}
}

// VerifierSignatureHashes sets the hashing algorithms to use
// when validating the "Signature" header. Hashes are tried
// in order, and the FIRST successful match returns success.
// If ALL hash attempts fail, then validation fails.
func VerifierSignatureHashes(hashes ...crypto.Hash) VerifierOption {
	return func(verifier *Verifier) {
		verifier.SignatureHashes = hashes
	}
}

// VerifierTimeout sets the maximum age of a request and
// signature (in seconds).  Messages received after this
// time duration will be rejected.
// Default is 43200 seconds (12 hours).
func VerifierTimeout(seconds int) VerifierOption {
	return func(verifier *Verifier) {
		verifier.Timeout = seconds
	}
}

// VerifierIgnoreTimeout sets the verifier to ignore
// message and signature time stamps.  This is useful
// for testing signatures, but should not be used in
// production.
func VerifierIgnoreTimeout() VerifierOption {
	return func(verifier *Verifier) {
		verifier.Timeout = 0
	}
}

// WithRefreshKey supplies a fallback finder that is consulted ONLY when a
// signature fails to verify against the key the primary finder returned. A
// caller that caches Actor documents uses this to notice that a peer has
// rotated its key, without re-fetching on every request.
func WithRefreshKey(refresh PublicKeyFinder) VerifierOption {

	// The refresh deliberately fires on a FAILURE rather than on a timer, so a peer that rotates is
	// repaired on its first rejected delivery instead of after a fixed window of bounced traffic.
	// The caller is responsible for bounding it: a sender can provoke this lookup at will, naming any
	// keyID host they like, so an unbounded refresh is an amplifier.
	return func(verifier *Verifier) {
		verifier.RefreshKey = refresh
	}
}

// VerifierIgnoreBodyDigest sets the verifier to ignore
// the "Digest" header.  This is useful for testing
// but should not be used in production.
func VerifierIgnoreBodyDigest() VerifierOption {
	return func(verifier *Verifier) {
		verifier.CheckDigest = false
	}
}

// VerifierMaxBodySize sets the largest body, in bytes, that the Digest check reads. Zero or
// less restores the default. Set it to the limit the caller already read the body under.
func VerifierMaxBodySize(maxBytes int64) VerifierOption {
	return func(verifier *Verifier) {
		verifier.MaxBodySize = maxBytes
	}
}

// WithHostname names the host that requests are signed for, in place of the request's own
// Host. A server behind a proxy that rewrites Host passes its public name. Empty changes nothing.
func WithHostname(hostname string) VerifierOption {
	return func(verifier *Verifier) {
		verifier.Hostname = hostname
	}
}
