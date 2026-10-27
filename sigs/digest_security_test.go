package sigs

import (
	"bytes"
	"crypto"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// newDigestRequest builds a request with the given body and applies a SHA-256
// digest header.
func newDigestRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	request, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader([]byte(body)))
	require.Nil(t, err)
	require.Nil(t, ApplyDigest(request, "SHA-256", DigestSHA256))
	return request
}

// TestVerifyDigest_Reject_Tampered confirms that a digest not matching the body
// is rejected.
func TestVerifyDigest_Reject_Tampered(t *testing.T) {

	request := newDigestRequest(t, `{"amount":"100"}`)

	// Swap the body without updating the digest.
	request.Body = readCloser(`{"amount":"999999"}`)

	err := VerifyDigest(request, crypto.SHA256)
	require.NotNil(t, err, "a digest that doesn't match the body must be rejected")
}

// TestVerifyDigest_Reject_DigestShopping confirms that if ANY provided digest
// fails to match, verification fails immediately, even if another digest in the
// header would match. This prevents an attacker from appending a valid digest
// alongside an invalid one.
func TestVerifyDigest_Reject_DigestShopping(t *testing.T) {

	body := `{"hello":"world"}`
	request := newDigestRequest(t, body)

	// Header contains a VALID sha-256 plus a BOGUS sha-512. The bogus one must
	// cause an immediate rejection.
	valid := "SHA-256=" + DigestSHA256([]byte(body))
	bogus := "SHA-512=AAAAINVALIDDIGESTAAAA="
	request.Header.Set(FieldDigest, valid+","+bogus)

	err := VerifyDigest(request, crypto.SHA256, crypto.SHA512)
	require.NotNil(t, err, "a single non-matching digest must fail the whole check")
}

// TestVerifyDigest_Reject_AlgorithmNotAllowed confirms that a digest which
// matches the body but uses an algorithm NOT in the allowed list is rejected.
func TestVerifyDigest_Reject_AlgorithmNotAllowed(t *testing.T) {

	body := `{"hello":"world"}`
	request, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader([]byte(body)))
	require.Nil(t, err)

	// Apply a valid SHA-256 digest...
	require.Nil(t, ApplyDigest(request, "SHA-256", DigestSHA256))

	// ...but only allow SHA-512. The SHA-256 digest matches the body but is not
	// in the allowed list, so there is no acceptable digest.
	err = VerifyDigest(request, crypto.SHA512)
	require.NotNil(t, err, "a matching digest with a disallowed algorithm must be rejected")
}

// TestVerifyDigest_UnknownAlgorithmSkipped confirms an unrecognized digest
// algorithm in the header is skipped, while a recognized matching one passes.
func TestVerifyDigest_UnknownAlgorithmSkipped(t *testing.T) {

	body := `{"hello":"world"}`
	request := newDigestRequest(t, body)

	// Prepend an unknown algorithm; the known SHA-256 still matches.
	valid := "SHA-256=" + DigestSHA256([]byte(body))
	request.Header.Set(FieldDigest, "MD5=ignored,"+valid)

	err := VerifyDigest(request, crypto.SHA256)
	require.Nil(t, err, "an unknown algorithm should be skipped, not fatal, when a valid digest is present")
}

// TestVerifyDigest_NoDigestHeader confirms that a request with no Digest header
// passes (there is nothing to verify).
func TestVerifyDigest_NoDigestHeader(t *testing.T) {

	request, err := http.NewRequest("GET", "https://example.com/inbox", nil)
	require.Nil(t, err)

	require.Nil(t, VerifyDigest(request, crypto.SHA256))
}

// TestVerifyDigest_SHA512 confirms a SHA-512 digest round-trips.
func TestVerifyDigest_SHA512(t *testing.T) {

	body := `{"hello":"world"}`
	request, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader([]byte(body)))
	require.Nil(t, err)

	require.Nil(t, ApplyDigest(request, "SHA-512", DigestSHA512))
	require.Nil(t, VerifyDigest(request, crypto.SHA512))
}

// TestVerifyDigest_NilRequest confirms a nil request is rejected without
// panicking.
func TestVerifyDigest_NilRequest(t *testing.T) {
	require.NotNil(t, VerifyDigest(nil, crypto.SHA256))
}

// TestApplyDigest_NilRequest confirms ApplyDigest rejects a nil request.
func TestApplyDigest_NilRequest(t *testing.T) {
	require.NotNil(t, ApplyDigest(nil, "SHA-256", DigestSHA256))
}

// TestApplyDigest_EmptyBody confirms ApplyDigest is a no-op (no header set) when
// the body is empty.
func TestApplyDigest_EmptyBody(t *testing.T) {

	request, err := http.NewRequest("GET", "https://example.com/inbox", nil)
	require.Nil(t, err)

	require.Nil(t, ApplyDigest(request, "SHA-256", DigestSHA256))
	require.Empty(t, request.Header.Get(FieldDigest), "no digest header should be set for an empty body")
}

// TestDigestFuncs_KnownVectors locks the digest functions to known-answer
// vectors so a change in algorithm output is caught immediately.
func TestDigestFuncs_KnownVectors(t *testing.T) {

	// echo -n "abc" | openssl dgst -sha256 -binary | base64
	require.Equal(t, "ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=", DigestSHA256([]byte("abc")))

	// echo -n "abc" | openssl dgst -sha512 -binary | base64
	require.Equal(t,
		"3a81oZNherrMQXNJriBBMRLm+k6JqX6iCp7u5ktV05ohkpkqJ0/BqDa6PCOj/uu9RU1EI2Q86A4qmslPpUyknw==",
		DigestSHA512([]byte("abc")))

	// Empty input has a well-known SHA-256 digest.
	require.Equal(t, "47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=", DigestSHA256([]byte("")))
}
