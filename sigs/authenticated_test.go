package sigs

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"testing"
	"time"

	"github.com/benpate/remote"
	"github.com/stretchr/testify/require"
)

// TestGetAuthenticatedActor confirms a validly signed request yields the actor
// ID (the keyID with its fragment stripped).
func TestGetAuthenticatedActor(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	keyID := "https://example.com/users/alice#main-key"
	request := newSignedRequest(t, `{"hello":"world"}`, keyID, privateKey)

	// A valid signature resolves to the actor ID (fragment stripped).
	actorID := GetAuthenticatedActor(request, rsaKeyFinder(privateKey))
	require.Equal(t, "https://example.com/users/alice", actorID)
}

// TestGetAuthenticatedActor_Invalid confirms a request that fails verification
// yields an empty actor ID rather than an error or panic.
func TestGetAuthenticatedActor_Invalid(t *testing.T) {

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := newSignedRequest(t, `{"hello":"world"}`, "https://example.com/users/alice#main-key", signingKey)

	// Verified against the WRONG key -> no authenticated actor.
	actorID := GetAuthenticatedActor(request, rsaKeyFinder(otherKey))
	require.Equal(t, "", actorID)
}

// TestWithSigner confirms the remote.Option produced by WithSigner signs an outbound request, copies
// its Digest and Signature into the transaction, and that the signed request verifies
func TestWithSigner(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	signer := NewSigner("https://example.com/users/alice#main-key", privateKey)

	// Build an outbound request and run it through the middleware's ModifyRequest.
	request, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader([]byte(`{"hello":"world"}`)))
	require.Nil(t, err)
	request.Header.Set("Date", dateHeader(time.Now()))

	option := WithSigner(signer)
	require.NotNil(t, option.ModifyRequest)

	// ModifyRequest returns nil (it does not replace the request) and signs in
	// place; it also writes the headers back into the transaction.
	txn := remote.Post("https://example.com/inbox")
	response := option.ModifyRequest(txn, request)
	require.Nil(t, response)

	// The middleware must have applied both headers.
	require.NotEmpty(t, request.Header.Get("Signature"))
	require.NotEmpty(t, request.Header.Get("Digest"))

	// ...and copied the same values into the transaction.
	transactionHeaders := txn.MarshalMap()["header"]
	require.Equal(t, map[string]string{
		"Digest":    request.Header.Get("Digest"),
		"Signature": request.Header.Get("Signature"),
	}, transactionHeaders)

	// And the signed request must verify.
	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.Nil(t, err)
}
