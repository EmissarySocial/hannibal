package sigs

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

/******************************************
 * Happy-Path Round Trips (RSA + ECDSA, SHA-256 + SHA-512)
 ******************************************/

// TestVerify_RoundTrip_RSA confirms a freshly signed RSA request verifies for
// both SHA-256 and SHA-512 signature hashes.
func TestVerify_RoundTrip_RSA(t *testing.T) {

	check := func(name string, hash crypto.Hash) {
		t.Run(name, func(t *testing.T) {
			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			require.Nil(t, err)

			request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey,
				SignerSignatureHash(hash), SignerBodyDigest(hash))

			signature, err := Verify(request, rsaKeyFinder(privateKey))
			require.Nil(t, err, "a correctly signed request must verify")
			require.Equal(t, "rsa-key", signature.KeyID)
		})
	}

	check("SHA-256", crypto.SHA256)
	check("SHA-512", crypto.SHA512)
}

// TestVerify_RoundTrip_ECDSA confirms a freshly signed ECDSA request verifies.
// ECDSA verification was previously untested in this package.
func TestVerify_RoundTrip_ECDSA(t *testing.T) {

	check := func(name string, curve elliptic.Curve, hash crypto.Hash) {
		t.Run(name, func(t *testing.T) {
			privateKey, err := ecdsa.GenerateKey(curve, rand.Reader)
			require.Nil(t, err)

			publicPEM := encodeECDSAPublicPEM(t, &privateKey.PublicKey)
			keyFinder := func(_ string) (string, error) { return publicPEM, nil }

			request := newSignedRequest(t, `{"hello":"ecdsa"}`, "ecdsa-key", privateKey,
				SignerSignatureHash(hash), SignerBodyDigest(hash))

			_, err = Verify(request, keyFinder)
			require.Nil(t, err, "a correctly signed ECDSA request must verify")
		})
	}

	check("P256-SHA256", elliptic.P256(), crypto.SHA256)
	check("P521-SHA512", elliptic.P521(), crypto.SHA512)
}

/******************************************
 * Rejection Paths -- the security-critical cases
 ******************************************/

// TestVerify_Reject_TamperedBody confirms that modifying the body after signing
// (without updating the Digest) is rejected by the digest check.
func TestVerify_Reject_TamperedBody(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"amount":"100"}`, "rsa-key", privateKey)

	// Attacker swaps the body but leaves the signed Digest header in place.
	request.Body = readCloser(`{"amount":"999999"}`)
	request.ContentLength = int64(len(`{"amount":"999999"}`))

	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.NotNil(t, err, "a tampered body must be rejected")
}

// TestVerify_Reject_TamperedDigestHeader confirms that altering the Digest header
// to match a tampered body still fails, because the signature covers the digest.
func TestVerify_Reject_TamperedDigestHeader(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	newBody := `{"amount":"999999"}`
	request := newSignedRequest(t, `{"amount":"100"}`, "rsa-key", privateKey)

	// Attacker rewrites BOTH the body and the Digest header to match it. The
	// digest now passes, but the signature was computed over the old digest, so
	// the signature check must fail.
	request.Body = readCloser(newBody)
	request.ContentLength = int64(len(newBody))
	request.Header.Set(FieldDigest, "SHA-256="+DigestSHA256([]byte(newBody)))

	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.NotNil(t, err, "a re-digested tampered body must still fail the signature check")
}

// TestVerify_Reject_TamperedSignature confirms that flipping bytes in the
// signature is rejected.
func TestVerify_Reject_TamperedSignature(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey)

	// Parse the signature, corrupt one byte, and write it back.
	signature, err := ParseSignature(GetSignature(request))
	require.Nil(t, err)
	signature.Signature[0] ^= 0xFF
	request.Header.Set("Signature", signature.String())

	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.NotNil(t, err, "a corrupted signature must be rejected")
}

// TestVerify_Reject_WrongKey confirms that a signature verified against a
// different public key than the one that signed it is rejected.
func TestVerify_Reject_WrongKey(t *testing.T) {

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", signingKey)

	// The key finder returns the WRONG public key.
	_, err = Verify(request, rsaKeyFinder(otherKey))
	require.NotNil(t, err, "verifying against the wrong public key must fail")
}

// TestVerify_Reject_ExpiredByExpires confirms a signature whose explicit
// "expires" timestamp is in the past is rejected.
func TestVerify_Reject_ExpiredByExpires(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	// Expires one hour ago.
	expiredAt := time.Now().Add(-1 * time.Hour).Unix()
	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey,
		SignerExpires(expiredAt))

	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.NotNil(t, err, "a signature past its Expires time must be rejected")
}

// TestVerify_Reject_ExpiredDateHeader confirms a Date header older than the
// verifier's timeout window is rejected.
func TestVerify_Reject_ExpiredDateHeader(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey)

	// Backdate the request well beyond the default 12-hour window. The signature
	// covers the date, so we must re-sign after changing it.
	request.Header.Set("Date", dateHeader(time.Now().Add(-48*time.Hour)))
	require.Nil(t, Sign(request, "rsa-key", privateKey))

	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.NotNil(t, err, "a request older than the timeout window must be rejected")
}

// TestVerify_Reject_MalformedDateHeader confirms an unparseable Date header is
// rejected rather than silently accepted.
func TestVerify_Reject_MalformedDateHeader(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey)
	request.Header.Set("Date", "not-a-real-date")

	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.NotNil(t, err, "a malformed Date header must be rejected")
}

// TestVerify_Reject_MissingRequiredField confirms the verifier rejects a
// signature that omits a header the verifier requires.
func TestVerify_Reject_MissingRequiredField(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	// Sign covering only (request-target) and host -- no date, no digest.
	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey,
		SignerFields(FieldRequestTarget, FieldHost))

	// Require the digest field that the signature does not include.
	_, err = Verify(request, rsaKeyFinder(privateKey),
		VerifierFields(FieldRequestTarget, FieldHost, FieldDigest))
	require.NotNil(t, err, "a signature missing a required field must be rejected")
}

// TestVerify_Reject_KeyFinderError confirms that a key-finder error aborts
// verification.
func TestVerify_Reject_KeyFinderError(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey)

	failingFinder := func(_ string) (string, error) {
		return "", errString("key not found")
	}

	_, err = Verify(request, failingFinder)
	require.NotNil(t, err, "a key-finder error must abort verification")
}

// TestVerify_Reject_BadPEMFromKeyFinder confirms an undecodable PEM from the key
// finder is rejected.
func TestVerify_Reject_BadPEMFromKeyFinder(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"hello":"world"}`, "rsa-key", privateKey)

	badFinder := func(_ string) (string, error) {
		return "this is not a PEM", nil
	}

	_, err = Verify(request, badFinder)
	require.NotNil(t, err, "an undecodable public key must be rejected")
}

// TestVerify_Reject_NoSignatureHeader confirms a request with no Signature header
// is rejected at parse time.
func TestVerify_Reject_NoSignatureHeader(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request, err := http.NewRequest("GET", "https://example.com/inbox", nil)
	require.Nil(t, err)
	request.Header.Set("Date", dateHeader(time.Now()))

	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.NotNil(t, err, "a request with no signature must be rejected")
}

// TestVerify_Reject_NilRequest confirms a nil request is rejected without
// panicking, for both the package function and the method.
func TestVerify_Reject_NilRequest(t *testing.T) {

	keyFinder := func(_ string) (string, error) { return "", nil }

	_, err := Verify(nil, keyFinder)
	require.NotNil(t, err)

	verifier := NewVerifier()
	_, err = verifier.Verify(nil, keyFinder)
	require.NotNil(t, err)
}

/******************************************
 * Low-Level verifySignature Rejections
 ******************************************/

// TestVerifySignature_UnrecognizedKeyType confirms verifySignature rejects a key
// that is neither RSA nor ECDSA.
func TestVerifySignature_UnrecognizedKeyType(t *testing.T) {

	// A plain string is not a recognized public key type.
	err := verifySignature("not-a-key", crypto.SHA256, []byte("digest"), []byte("sig"))
	require.NotNil(t, err, "an unrecognized key type must be rejected")
}

// TestVerifySignature_ECDSA_Tampered confirms a tampered ECDSA signature fails
// the low-level check.
func TestVerifySignature_ECDSA_Tampered(t *testing.T) {

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)

	digest := []byte("0123456789abcdef0123456789abcdef")
	signature, err := makeSignedDigest(digest, crypto.SHA256, privateKey)
	require.Nil(t, err)

	// Unmodified signature verifies.
	require.Nil(t, verifySignature(&privateKey.PublicKey, crypto.SHA256, digest, signature))

	// Corrupting a byte makes it fail.
	signature[0] ^= 0xFF
	require.NotNil(t, verifySignature(&privateKey.PublicKey, crypto.SHA256, digest, signature))
}

/******************************************
 * Test Helpers
 ******************************************/

// newSignedRequest builds a POST request with the given body, signs it with the
// provided private key, and returns the request. The body digest and signature
// are applied, so the request is ready to verify.
func newSignedRequest(t *testing.T, body string, publicKeyID string, privateKey crypto.PrivateKey, options ...SignerOption) *http.Request {
	t.Helper()

	request, err := http.NewRequest("POST", "https://example.com/inbox?x=1", bytes.NewReader([]byte(body)))
	require.Nil(t, err)
	request.Header.Set("Content-Type", "application/activity+json")
	request.Header.Set("Date", dateHeader(time.Now()))

	err = Sign(request, publicKeyID, privateKey, options...)
	require.Nil(t, err)

	return request
}

// rsaKeyFinder returns a PublicKeyFinder that always returns the RSA public key
// belonging to the given private key.
func rsaKeyFinder(privateKey *rsa.PrivateKey) PublicKeyFinder {
	return func(_ string) (string, error) {
		return EncodePublicPEM(privateKey), nil
	}
}

// encodeECDSAPublicPEM encodes an ECDSA public key as a PKIX "PUBLIC KEY" PEM,
// which DecodePublicPEM accepts. The package only ships an RSA PEM encoder, so
// the ECDSA tests encode their own keys here.
func encodeECDSAPublicPEM(t *testing.T, publicKey *ecdsa.PublicKey) string {
	t.Helper()

	der, err := x509.MarshalPKIXPublicKey(publicKey)
	require.Nil(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// readCloser wraps a string as a replayable request body.
func readCloser(body string) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(body)))
}

// errString is a minimal error type for tests that need a custom error.
type errString string

// Error returns the error's text
func (e errString) Error() string { return string(e) }
