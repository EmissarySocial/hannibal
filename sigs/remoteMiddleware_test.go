package sigs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/remote"
	"github.com/stretchr/testify/require"
)

// TestWithSigner_RefusesWhatItCannotSign verifies that a request is never sent unsigned
func TestWithSigner_RefusesWhatItCannotSign(t *testing.T) {

	received := 0

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		received++
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// A string is not a key that this package can sign with
	signer := NewSigner("https://example.com/@alice#main-key", "not a key")

	err := remote.Post(server.URL).AllowPrivateIPs(true).With(WithSigner(signer)).Body("{}").Send()

	require.Error(t, err)
	require.True(t, derp.IsClientError(err), "a caller must not retry this")
	require.Zero(t, received)
}

// TestWithSigner_Signs verifies that a working key still sends a signed request
func TestWithSigner_Signs(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var signature, digest string

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		signature = request.Header.Get("Signature")
		digest = request.Header.Get("Digest")
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	signer := NewSigner("https://example.com/@alice#main-key", privateKey)
	require.NoError(t, remote.Post(server.URL).AllowPrivateIPs(true).With(WithSigner(signer)).Body("{}").Send())
	require.Contains(t, signature, `keyId="https://example.com/@alice#main-key"`)
	require.NotEmpty(t, digest)
}

// TestWithSigner_SignsOnce verifies that the option signs in one hook only, so that one
// request costs one signature
func TestWithSigner_SignsOnce(t *testing.T) {

	option := WithSigner(NewSigner("https://example.com/@alice#main-key", "not a key"))

	require.Nil(t, option.BeforeRequest)
	require.NotNil(t, option.ModifyRequest)
}

// signTrial signs a request that is never sent, and returns the Signer's error
func signTrial(t *testing.T, privateKey crypto.PrivateKey) error {

	t.Helper()

	request, err := http.NewRequest(http.MethodPost, "https://example.com/inbox", strings.NewReader("{}"))
	require.NoError(t, err)

	signer := NewSigner("https://example.com/@alice#main-key", privateKey)
	return signer.Sign(request)
}

// TestSigner_Sign_Keys verifies which keys can sign
func TestSigner_Sign_Keys(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	require.NoError(t, signTrial(t, privateKey))
	require.Error(t, signTrial(t, "not a key"))
	require.Error(t, signTrial(t, nil))
	require.Error(t, signTrial(t, (*rsa.PrivateKey)(nil)), "a nil key must not panic")
	require.Error(t, signTrial(t, (*ecdsa.PrivateKey)(nil)), "a nil key must not panic")
}

// TestSigner_Sign_NeverReportsTheKey verifies that a key of an unknown type is named by its
// type, and stays out of the error
func TestSigner_Sign_NeverReportsTheKey(t *testing.T) {

	err := signTrial(t, []byte("SECRET-KEY-MATERIAL"))
	require.Error(t, err)

	encoded, marshalErr := json.Marshal(err)
	require.NoError(t, marshalErr)
	require.NotContains(t, string(encoded)+err.Error(), "SECRET-KEY-MATERIAL")
	require.NotContains(t, string(encoded), "U0VDUkVU", "the key in base64")
}
