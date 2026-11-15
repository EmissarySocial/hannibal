package sigs

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// defectKey is one RSA key for the tests in this file
func defectKey(t *testing.T) (*rsa.PrivateKey, PublicKeyFinder) {

	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return privateKey, func(string) (string, error) {
		return EncodePublicPEM(privateKey), nil
	}
}

// TestVerify_Hostname verifies that the caller, and never the client, chooses the host a signature is checked against
func TestVerify_Hostname(t *testing.T) {

	privateKey, keyFinder := defectKey(t)

	// A request signed for victim.example, replayed to attacker.example with the header added
	signed := httptest.NewRequest(http.MethodPost, "https://victim.example/inbox", strings.NewReader(`{"a":1}`))
	require.NoError(t, Sign(signed, "https://example.com/@alice#main-key", privateKey))

	replayed := httptest.NewRequest(http.MethodPost, "https://attacker.example/inbox", strings.NewReader(`{"a":1}`))
	replayed.Header = signed.Header.Clone()
	replayed.Header.Set("X-Forwarded-Host", "victim.example")

	_, err := Verify(replayed, keyFinder)
	require.Error(t, err, "the header must never be believed")

	// A server behind a proxy names its own public host
	replayed.Body = http.NoBody
	replayed = httptest.NewRequest(http.MethodPost, "https://attacker.example/inbox", strings.NewReader(`{"a":1}`))
	replayed.Header = signed.Header.Clone()
	replayed.Header.Set("X-Forwarded-Host", "victim.example")

	_, err = Verify(replayed, keyFinder, WithHostname("victim.example"))
	require.NoError(t, err)

	// An empty name leaves the request's own Host in place
	replayed = httptest.NewRequest(http.MethodPost, "https://attacker.example/inbox", strings.NewReader(`{"a":1}`))
	replayed.Header = signed.Header.Clone()
	replayed.Header.Set("X-Forwarded-Host", "victim.example")

	_, err = Verify(replayed, keyFinder, WithHostname(""))
	require.Error(t, err)

	// A name that the request was not signed for fails, even on the right server
	direct := httptest.NewRequest(http.MethodPost, "https://victim.example/inbox", strings.NewReader(`{"a":1}`))
	direct.Header = signed.Header.Clone()

	_, err = Verify(direct, keyFinder, WithHostname("other.example"))
	require.Error(t, err)
}

// TestVerify_Date verifies that a request must carry a date that is neither old nor in the future
func TestVerify_Date(t *testing.T) {

	privateKey, keyFinder := defectKey(t)

	verifyWithDate := func(date string) error {

		request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", strings.NewReader(`{"a":1}`))
		request.Header.Set("Date", date)

		signer := NewSigner("https://example.com/@alice#main-key", privateKey)

		// Build the signature by hand, so that Sign cannot repair the date
		require.NoError(t, ApplyDigest(request, "SHA-256", mustDigestFunc(t)))
		signature := NewSignature()
		plaintext := makePlaintext(request, signature, "", signer.Fields...)
		hashed, err := makeSignatureHash(plaintext, signer.SignatureHash)
		require.NoError(t, err)
		signed, err := makeSignedDigest(hashed, signer.SignatureHash, privateKey)
		require.NoError(t, err)

		signature.KeyID = signer.PublicKeyID
		signature.Headers = signer.Fields
		signature.Algorithm = getAlgorithmName(privateKey, signer.SignatureHash)
		signature.Signature = signed
		request.Header.Set("Signature", signature.String())

		_, err = Verify(request, keyFinder)
		return err
	}

	require.NoError(t, verifyWithDate(dateHeader(time.Now())))
	require.NoError(t, verifyWithDate(dateHeader(time.Now().Add(time.Hour))), "an hour of clock skew is allowed")
	require.Error(t, verifyWithDate(dateHeader(time.Now().Add(-13*time.Hour))))
	require.Error(t, verifyWithDate(dateHeader(time.Now().Add(13*time.Hour))), "a date in the future")
	require.Error(t, verifyWithDate(dateHeader(time.Now().AddDate(10, 0, 0))), "a date far in the future")
	require.Error(t, verifyWithDate(""), "no date at all")
}

// mustDigestFunc returns the SHA-256 digest function
func mustDigestFunc(t *testing.T) DigestFunc {

	t.Helper()

	result, err := getDigestFuncByName("SHA-256")
	require.NoError(t, err)

	return result
}

// TestVerify_BodyWithoutDigest verifies that a signature made over a request with no body
// cannot carry a body
func TestVerify_BodyWithoutDigest(t *testing.T) {

	privateKey, keyFinder := defectKey(t)

	signed := httptest.NewRequest(http.MethodGet, "https://example.com/inbox", nil)
	require.NoError(t, Sign(signed, "https://example.com/@alice#main-key", privateKey, SignerFields(FieldRequestTarget, FieldHost, FieldDate)))

	// The same signature, unchanged, still verifies
	_, err := Verify(signed, keyFinder)
	require.NoError(t, err)

	// With a body attached, it does not
	forged := httptest.NewRequest(http.MethodGet, "https://example.com/inbox", strings.NewReader(`{"type":"Delete"}`))
	forged.Header = signed.Header.Clone()

	_, err = Verify(forged, keyFinder)
	require.Error(t, err)

	require.Error(t, VerifyDigest(forged))
	require.NoError(t, VerifyDigest(httptest.NewRequest(http.MethodGet, "https://example.com/inbox", nil)))
}

// TestVerify_LargeBody verifies that a body over the default limit verifies when the caller
// raises the limit, and not otherwise
func TestVerify_LargeBody(t *testing.T) {

	privateKey, keyFinder := defectKey(t)

	body := bytes.Repeat([]byte("a"), (1<<20)+10)
	digest := mustDigestFunc(t)(body)

	newRequest := func() *http.Request {

		request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", bytes.NewReader(body))
		request.Header.Set("Digest", "SHA-256="+digest)
		request.Header.Set("Date", dateHeader(time.Now()))

		signer := NewSigner("https://example.com/@alice#main-key", privateKey, SignerFields(FieldRequestTarget, FieldHost, FieldDate))
		signer.Fields = []string{FieldRequestTarget, FieldHost, FieldDate, FieldDigest}

		signature := NewSignature()
		plaintext := makePlaintext(request, signature, "", signer.Fields...)
		hashed, err := makeSignatureHash(plaintext, signer.SignatureHash)
		require.NoError(t, err)
		signed, err := makeSignedDigest(hashed, signer.SignatureHash, privateKey)
		require.NoError(t, err)

		signature.KeyID = signer.PublicKeyID
		signature.Headers = signer.Fields
		signature.Algorithm = getAlgorithmName(privateKey, signer.SignatureHash)
		signature.Signature = signed
		request.Header.Set("Signature", signature.String())

		return request
	}

	_, err := Verify(newRequest(), keyFinder)
	require.Error(t, err, "over the default limit")

	_, err = Verify(newRequest(), keyFinder, VerifierMaxBodySize(2<<20))
	require.NoError(t, err)

	_, err = Verify(newRequest(), keyFinder, VerifierMaxBodySize(0))
	require.Error(t, err, "zero restores the default")
}

// TestSign_Created verifies that a signature over (created) can be verified
func TestSign_Created(t *testing.T) {

	privateKey, keyFinder := defectKey(t)

	request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", strings.NewReader(`{"a":1}`))

	signer := NewSigner("https://example.com/@alice#main-key", privateKey,
		SignerFields(FieldRequestTarget, FieldCreated, FieldHost, FieldDate, FieldDigest))
	signer.Created = time.Now().Unix()

	require.NoError(t, signer.Sign(request))
	require.Contains(t, request.Header.Get("Signature"), "created=")

	_, err := Verify(request, keyFinder)
	require.NoError(t, err)
}

// TestVerify_EncodedPath verifies that a signature for an encoded path does not verify on the decoded one
func TestVerify_EncodedPath(t *testing.T) {

	privateKey, keyFinder := defectKey(t)

	signed := httptest.NewRequest(http.MethodPost, "https://example.com/a%2Fb/inbox", strings.NewReader(`{"a":1}`))
	require.NoError(t, Sign(signed, "https://example.com/@alice#main-key", privateKey))

	// The request verifies on the path it was signed for
	_, err := Verify(signed, keyFinder)
	require.NoError(t, err)

	moved := httptest.NewRequest(http.MethodPost, "https://example.com/a/b/inbox", strings.NewReader(`{"a":1}`))
	moved.Header = signed.Header.Clone()

	_, err = Verify(moved, keyFinder)
	require.Error(t, err)

	require.Equal(t, "/a%2Fb/inbox?x=1", getPathAndQuery(httptest.NewRequest(http.MethodGet, "https://example.com/a%2Fb/inbox?x=1", nil).URL))
	require.Equal(t, "/", getPathAndQuery(httptest.NewRequest(http.MethodGet, "https://example.com", nil).URL))
}

// TestSigner_MaxBodySize verifies that a body over the limit cannot be signed, and that the
// limit can be raised
func TestSigner_MaxBodySize(t *testing.T) {

	privateKey, keyFinder := defectKey(t)
	body := strings.Repeat("a", (1<<20)+1)

	signWith := func(options ...SignerOption) (*http.Request, error) {
		request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", strings.NewReader(body))
		signer := NewSigner("https://example.com/@alice#main-key", privateKey, options...)
		return request, signer.Sign(request)
	}

	// One byte over the default is refused, with no Digest left behind
	request, err := signWith()
	require.Error(t, err)
	require.Empty(t, request.Header.Get("Digest"))

	// A lower limit is honored too
	_, err = signWith(SignerMaxBodySize(16))
	require.Error(t, err)

	// A raised limit signs it, and a verifier with the same limit accepts it
	request, err = signWith(SignerMaxBodySize(1 << 21))
	require.NoError(t, err)

	_, err = Verify(request, keyFinder, VerifierMaxBodySize(1<<21))
	require.NoError(t, err)
}
