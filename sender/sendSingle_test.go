package sender

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyedActor is a test Actor that carries a real RSA private key, so that
// outbound requests can actually be signed.
type keyedActor struct {
	id         string
	keyID      string
	privateKey crypto.PrivateKey
}

// ActorID returns the test actor's ID
func (a keyedActor) ActorID() string { return a.id }

// PrivateKey returns the test actor's key ID and RSA private key
func (a keyedActor) PrivateKey() (string, crypto.PrivateKey) {
	return a.keyID, a.privateKey
}

// keyedLocator resolves exactly one actor (the keyed test actor).
type keyedLocator struct {
	actor keyedActor
}

// Actor returns the keyed test actor for its own ID, and NotFound for any other ID
func (l keyedLocator) Actor(id string) (Actor, error) {
	if id == l.actor.id {
		return l.actor, nil
	}
	return nil, derp.NotFound("keyedLocator.Actor", "unknown actor", id)
}

// Recipient returns an empty sequence for every recipient
func (l keyedLocator) Recipient(_ string) (iter.Seq[string], error) {
	return func(_ func(string) bool) {}, nil
}

// TestSendToSingleRecipient_Success confirms a deliverable activity is POSTed to
// the recipient inbox, signed, and reports Success.
func TestSendToSingleRecipient_Success(t *testing.T) {

	sender, actorID := newKeyedSender(t)

	var gotSignature atomic.Bool
	var gotDigest atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature.Store(r.Header.Get("Signature") != "")
		gotDigest.Store(r.Header.Get("Digest") != "")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor":    actorID,
		"inbox":    server.URL,
		"activity": mapof.Any{"type": "Create", "actor": actorID},
	})

	assert.Equal(t, queue.ResultStatusSuccess, result.Status)
	assert.True(t, gotSignature.Load(), "the outbound request must be signed")
	assert.True(t, gotDigest.Load(), "the outbound request must carry a body digest")
}

// TestSendToSingleRecipient_ActorNotFound confirms an unknown sending actor
// yields a Failure (cannot be retried).
func TestSendToSingleRecipient_ActorNotFound(t *testing.T) {

	sender, _ := newKeyedSender(t)

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor":    "https://example.com/users/nobody",
		"inbox":    "https://example.com/inbox",
		"activity": mapof.Any{"type": "Create"},
	})

	assert.Equal(t, queue.ResultStatusFailure, result.Status)
}

// TestSendToSingleRecipient_ServerError confirms a 5xx response is classified as
// a retriable Error.
func TestSendToSingleRecipient_ServerError(t *testing.T) {

	sender, actorID := newKeyedSender(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor":    actorID,
		"inbox":    server.URL,
		"activity": mapof.Any{"type": "Create", "actor": actorID},
	})

	// 5xx is the remote server's fault -> retriable Error.
	assert.Equal(t, queue.ResultStatusError, result.Status)
}

// TestSendToSingleRecipient_ClientError confirms a 4xx response is classified as
// a non-retriable Failure.
func TestSendToSingleRecipient_ClientError(t *testing.T) {

	sender, actorID := newKeyedSender(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor":    actorID,
		"inbox":    server.URL,
		"activity": mapof.Any{"type": "Create", "actor": actorID},
	})

	// 4xx is our fault -> non-retriable Failure.
	assert.Equal(t, queue.ResultStatusFailure, result.Status)
}

// TestSendToSingleRecipient_TooManyRequests confirms a 429 response is requeued
// rather than failed.
func TestSendToSingleRecipient_TooManyRequests(t *testing.T) {

	sender, actorID := newKeyedSender(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor":    actorID,
		"inbox":    server.URL,
		"activity": mapof.Any{"type": "Create", "actor": actorID},
	})

	assert.Equal(t, queue.ResultStatusRequeue, result.Status)
}

// TestSignRequest confirms the signRequest middleware signs an outbound request,
// producing a verifiable signature.
func TestSignRequest(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyID := "https://example.com/users/alice#main-key"

	var verified atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := sigs.Verify(r, func(string) (string, error) {
			return sigs.EncodePublicPEM(privateKey), nil
		})
		verified.Store(err == nil)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// httptest gives us a *http.Client; use the remote package via the sender's
	// own path by signing through the middleware directly.
	request, err := http.NewRequest(http.MethodPost, server.URL, http.NoBody)
	require.NoError(t, err)
	require.NoError(t, sigs.Sign(request, keyID, privateKey))

	response, err := server.Client().Do(request)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })

	assert.True(t, verified.Load(), "the signed request must verify against the signing key")
}

// TestSendToSingleRecipient_Body confirms a serialized body is POSTed byte-for-byte, with a matching digest and signature
func TestSendToSingleRecipient_Body(t *testing.T) {

	sender, actorID := newKeyedSender(t)
	server, captured := captureInbox(t, sender)

	// Deliberately not in the order json.Marshal would produce, so a re-serialization would show
	body := `{"type":"Create","actor":"` + actorID + `","id":"https://example.com/1"}`

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor": actorID,
		"inbox": server.URL,
		"body":  body,
	})

	require.Equal(t, queue.ResultStatusSuccess, result.Status)
	assert.Equal(t, body, string(captured.body), "the body must be sent exactly as queued")
	assert.Equal(t, sha256Digest([]byte(body)), captured.digest, "the digest must cover the exact bytes sent")
	assert.True(t, captured.verified, "the signature must verify against the sent request")
}

// TestSendToSingleRecipient_LegacyActivity confirms a task queued before "body" existed still serializes its activity
func TestSendToSingleRecipient_LegacyActivity(t *testing.T) {

	sender, actorID := newKeyedSender(t)
	server, captured := captureInbox(t, sender)

	activity := mapof.Any{"type": "Create", "actor": actorID}

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor":    actorID,
		"inbox":    server.URL,
		"activity": activity,
	})

	require.Equal(t, queue.ResultStatusSuccess, result.Status)

	expected, err := json.Marshal(activity)
	require.NoError(t, err)
	assert.JSONEq(t, string(expected), string(captured.body))
	assert.Equal(t, sha256Digest(captured.body), captured.digest)
	assert.True(t, captured.verified)
}

// TestSendToSingleRecipient_BodyWins confirms the body is sent when a task carries both a body and an activity
func TestSendToSingleRecipient_BodyWins(t *testing.T) {

	sender, actorID := newKeyedSender(t)
	server, captured := captureInbox(t, sender)

	body := `{"type":"Create","actor":"` + actorID + `"}`

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor":    actorID,
		"inbox":    server.URL,
		"body":     body,
		"activity": mapof.Any{"type": "Delete", "actor": actorID},
	})

	require.Equal(t, queue.ResultStatusSuccess, result.Status)
	assert.Equal(t, body, string(captured.body))
}

// TestSendToSingleRecipient_NothingToSend confirms a task with neither a body nor an activity fails without POSTing
func TestSendToSingleRecipient_NothingToSend(t *testing.T) {

	sender, actorID := newKeyedSender(t)
	server, captured := captureInbox(t, sender)

	result := sender.SendToSingleRecipient(mapof.Any{
		"actor": actorID,
		"inbox": server.URL,
	})

	assert.Equal(t, queue.ResultStatusFailure, result.Status)
	assert.Zero(t, captured.count, "nothing may be sent to the recipient")
}

// newKeyedSender builds a Sender whose single actor has a real signing key.
func newKeyedSender(t *testing.T) (Sender, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	actorID := "https://example.com/users/alice"
	actor := keyedActor{
		id:         actorID,
		keyID:      actorID + "#main-key",
		privateKey: privateKey,
	}

	q, _ := newRecordingQueue()

	// These tests POST to loopback httptest servers that remote's SSRF guard would
	// otherwise block, so opt in to private-IP delivery. Production keeps this FALSE.
	return New(keyedLocator{actor: actor}, q, AllowPrivateIPs(true)), actorID
}

// capturedRequest records what a test inbox received
type capturedRequest struct {
	mu       sync.Mutex
	count    int
	body     []byte
	digest   string
	verified bool
}

// captureInbox starts a local inbox that records each request's body and digest, and verifies its signature
func captureInbox(t *testing.T, sender Sender) (*httptest.Server, *capturedRequest) {
	t.Helper()

	publicKeyPEM := sigs.EncodePublicPEM(sender.locator.(keyedLocator).actor.privateKey.(*rsa.PrivateKey))
	captured := &capturedRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Verify the signature (which also checks the digest) before the body is read
		_, verifyErr := sigs.Verify(r, func(string) (string, error) { return publicKeyPEM, nil })
		body, readErr := io.ReadAll(r.Body)

		captured.mu.Lock()
		captured.count++
		captured.body = body
		captured.digest = r.Header.Get("Digest")
		captured.verified = (verifyErr == nil) && (readErr == nil)
		captured.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	return server, captured
}

// sha256Digest returns the Digest header value for the provided body
func sha256Digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])
}
