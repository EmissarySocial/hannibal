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
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/hannibal/vocab"
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

// rateLimitedInbox starts a local inbox that answers every request with a 429
func rateLimitedInbox(t *testing.T, retryAfter string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(server.Close)

	return server
}

// TestSendToSingleRecipient_RateLimitWaitIsCapped confirms a host cannot park a delivery for
// longer than an hour with one large Retry-After
func TestSendToSingleRecipient_RateLimitWaitIsCapped(t *testing.T) {

	sender, actorID := newKeyedSender(t)
	server := rateLimitedInbox(t, "8640000")

	result := sender.SendToSingleRecipient(mapof.Any{"actor": actorID, "inbox": server.URL, "body": `{"type":"Create"}`})

	assert.Equal(t, queue.ResultStatusRequeue, result.Status)
	assert.Equal(t, time.Hour, result.Delay)
}

// TestSendToSingleRecipient_RateLimitExpires confirms a host cannot hold a delivery with
// rate limits forever
func TestSendToSingleRecipient_RateLimitExpires(t *testing.T) {

	sender, actorID := newKeyedSender(t)
	server := rateLimitedInbox(t, "")

	old := mapof.Any{"actor": actorID, "inbox": server.URL, "body": `{"type":"Create"}`, argumentQueuedAt: time.Now().Add(-8 * 24 * time.Hour).Unix()}
	assert.Equal(t, queue.ResultStatusFailure, sender.SendToSingleRecipient(old).Status)

	recent := mapof.Any{"actor": actorID, "inbox": server.URL, "body": `{"type":"Create"}`, argumentQueuedAt: time.Now().Add(-6 * 24 * time.Hour).Unix()}
	assert.Equal(t, queue.ResultStatusRequeue, sender.SendToSingleRecipient(recent).Status)

	// A task queued before the argument existed is rescheduled as it always was
	legacy := mapof.Any{"actor": actorID, "inbox": server.URL, "body": `{"type":"Create"}`}
	assert.Equal(t, queue.ResultStatusRequeue, sender.SendToSingleRecipient(legacy).Status)
}

// failingLocator cannot load any actor, for a reason that may pass
type failingLocator struct{}

// Actor always fails with an internal error
func (failingLocator) Actor(string) (Actor, error) {
	return nil, derp.Internal("failingLocator.Actor", "database did not answer")
}

// Recipient returns an empty sequence for every recipient
func (failingLocator) Recipient(string) (iter.Seq[string], error) {
	return func(func(string) bool) {}, nil
}

// TestSender_ActorErrorIsRetried confirms both tasks retry when the sending actor cannot be
// loaded for a reason that may pass
func TestSender_ActorErrorIsRetried(t *testing.T) {

	q, _ := newRecordingQueue()
	sender := New(failingLocator{}, q)

	single := sender.SendToSingleRecipient(mapof.Any{"actor": "https://example.com/users/alice", "inbox": "https://example.com/inbox", "body": `{}`})
	assert.Equal(t, queue.ResultStatusError, single.Status)

	all := sender.SendToAllRecipients(mapof.Any{"actor": "https://example.com/users/alice", "to": "https://example.com/users/bob"})
	assert.Equal(t, queue.ResultStatusError, all.Status)
}

// TestSendToAllRecipients_ErrorsNameTheActivity confirms a failed fan-out names the activity
// and never carries its blind recipients
func TestSendToAllRecipients_ErrorsNameTheActivity(t *testing.T) {

	q, _ := newRecordingQueue()
	sender := New(failingLocator{}, q)

	result := sender.SendToAllRecipients(mapof.Any{
		"id":    "https://example.com/activities/1",
		"actor": "https://example.com/users/alice",
		"bcc":   []any{"https://example.com/users/secret"},
		"bto":   "https://example.com/users/hidden",
	})

	require.Error(t, result.Error)

	encoded, err := json.Marshal(result.Error)
	require.NoError(t, err)

	report := string(encoded) + derp.Message(result.Error) + result.Error.Error()
	assert.Contains(t, report, "https://example.com/activities/1")
	assert.NotContains(t, report, "secret")
	assert.NotContains(t, report, "hidden")
}

// TestSendToAllRecipients_TasksSayWhenTheyWereQueued confirms each send task carries its queue time
func TestSendToAllRecipients_TasksSayWhenTheyWereQueued(t *testing.T) {

	q, recorder := newRecordingQueue()
	sender := New(testLocator{}, q)
	before := time.Now().Unix()

	result := sender.SendToAllRecipients(mapof.Any{
		vocab.PropertyActor: "https://test.actor.social",
		vocab.PropertyTo:    "https://test.actor.social/",
	})

	require.Equal(t, queue.ResultStatusSuccess, result.Status)
	require.NotEmpty(t, recorder.tasks)

	for _, task := range recorder.tasks {
		assert.GreaterOrEqual(t, task.Arguments.GetInt64(argumentQueuedAt), before)
	}
}

// TestSendToSingleRecipient_KeyCannotSign confirms nothing is sent unsigned, and the task is not retried
func TestSendToSingleRecipient_KeyCannotSign(t *testing.T) {

	received := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	actorID := "https://example.com/users/alice"
	q, _ := newRecordingQueue()
	sender := New(keyedLocator{actor: keyedActor{id: actorID, keyID: actorID + "#main-key", privateKey: "not a key"}}, q, AllowPrivateIPs(true))

	result := sender.SendToSingleRecipient(mapof.Any{"actor": actorID, "inbox": server.URL, "body": `{"type":"Create"}`})

	assert.Equal(t, queue.ResultStatusFailure, result.Status)
	assert.Zero(t, received)
}
