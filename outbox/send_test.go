package outbox

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient is a streams.Client that resolves any recipient URI to a document
// whose inbox points at the configured inbox URL. This lets SendOne run fully
// offline: the "recipient lookup" is in-memory, and the actual POST goes to an
// httptest server.
type mockClient struct {
	inboxURL string
}

// SetRootClient is a NOOP for this stub
func (c mockClient) SetRootClient(streams.Client) {}

// Save is a NOOP for this stub
func (c mockClient) Save(streams.Document) error { return nil }

// Delete is a NOOP for this stub
func (c mockClient) Delete(string) error { return nil }

// Load returns a Person with this URI as its ID, whose inbox is the configured inbox URL
func (c mockClient) Load(uri string, _ ...any) (streams.Document, error) {
	return streams.NewDocument(map[string]any{
		vocab.PropertyID:   uri,
		vocab.PropertyType: vocab.ActorTypePerson,
		vocab.PropertyInbox: map[string]any{
			vocab.PropertyID: c.inboxURL,
		},
	}), nil
}

/******************************************
 * Send filtering (offline -- no SendOne)
 ******************************************/

// TestSend_FiltersRecipients confirms Send never delivers to empty, public, or
// self recipients. With only those recipients, the inbox is never hit.
func TestSend_FiltersRecipients(t *testing.T) {

	recorder := newInboxRecorder(t)
	actor := newSendingActor(t, recorder)

	message := mapof.Any{vocab.PropertyType: vocab.ActivityTypeCreate}

	actor.Send(message, makeIterator(
		"",                      // empty -> skipped
		vocab.NamespaceASPublic, // public -> skipped
		actor.ActorID(),         // self -> skipped
	))

	assert.Equal(t, 0, recorder.count(), "filtered recipients must not be delivered to")
}

/******************************************
 * SendOne (mock client + httptest)
 ******************************************/

// TestSendOne confirms SendOne resolves the recipient inbox and POSTs the signed
// message to it.
func TestSendOne(t *testing.T) {

	recorder := newInboxRecorder(t)
	actor := newSendingActor(t, recorder)

	message := mapof.Any{
		vocab.PropertyType:  vocab.ActivityTypeCreate,
		vocab.PropertyActor: actor.ActorID(),
	}

	err := actor.SendOne("https://remote.example.com/users/bob", message)
	require.NoError(t, err)

	require.Equal(t, 1, recorder.count())
	assert.Equal(t, vocab.ActivityTypeCreate, recorder.lastBody().GetString(vocab.PropertyType))
}

// TestSend_DeliversToRealRecipient confirms Send delivers to a non-filtered
// recipient.
func TestSend_DeliversToRealRecipient(t *testing.T) {

	recorder := newInboxRecorder(t)
	actor := newSendingActor(t, recorder)

	message := mapof.Any{vocab.PropertyType: vocab.ActivityTypeCreate}
	actor.Send(message, makeIterator("https://remote.example.com/users/bob"))

	assert.Equal(t, 1, recorder.count())
}

/******************************************
 * Builders -- verify the constructed message (end to end)
 ******************************************/

// TestSendFollow confirms the Follow builder produces a well-formed Follow
// activity addressed to the followed actor.
func TestSendFollow(t *testing.T) {

	recorder := newInboxRecorder(t)
	actor := newSendingActor(t, recorder)

	actor.SendFollow("https://example.com/activities/follow-1", "https://remote.example.com/users/bob")

	require.Equal(t, 1, recorder.count())
	body := recorder.lastBody()
	assert.Equal(t, vocab.ActivityTypeFollow, body.GetString(vocab.PropertyType))
	assert.Equal(t, "https://example.com/users/alice", body.GetString(vocab.PropertyActor))
	assert.Equal(t, "https://remote.example.com/users/bob", body.GetString(vocab.PropertyObject))
}

// TestSendUndo is the regression guard for the actor.ActorID method-value bug:
// the Undo message's actor field must be the actor's URL string, and the message
// must marshal to JSON (a func() string would have failed to marshal entirely).
func TestSendUndo(t *testing.T) {

	recorder := newInboxRecorder(t)
	actor := newSendingActor(t, recorder)

	// The activity being undone, addressed to a remote recipient.
	activity := streams.NewDocument(map[string]any{
		vocab.PropertyID:   "https://example.com/activities/like-1",
		vocab.PropertyType: vocab.ActivityTypeLike,
		vocab.PropertyTo:   "https://remote.example.com/users/bob",
	})

	actor.SendUndo(activity)

	require.Equal(t, 1, recorder.count(), "Undo must be delivered (it previously failed to marshal)")
	body := recorder.lastBody()
	assert.Equal(t, vocab.ActivityTypeUndo, body.GetString(vocab.PropertyType))
	assert.Equal(t, "https://example.com/users/alice", body.GetString(vocab.PropertyActor),
		"the actor field must be the actor URL string, not a function value")
}

/******************************************
 * SignRequest
 ******************************************/

// TestSignRequest confirms the outbound request carries a Signature and Digest.
func TestSignRequest(t *testing.T) {

	var hasSignature, hasDigest bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasSignature = r.Header.Get("Signature") != ""
		hasDigest = r.Header.Get("Digest") != ""
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	actor := NewActor("https://example.com/users/alice", privateKey,
		WithClient(mockClient{inboxURL: server.URL}),
		// This test POSTs to a loopback httptest server that remote's SSRF guard
		// would otherwise block.
		WithAllowPrivateIPs(true))

	err = actor.SendOne("https://remote.example.com/users/bob", mapof.Any{
		vocab.PropertyType: vocab.ActivityTypeCreate,
	})
	require.NoError(t, err)

	assert.True(t, hasSignature, "outbound request must be signed")
	assert.True(t, hasDigest, "outbound request must carry a body digest")
}

/******************************************
 * Test Helpers
 ******************************************/

// inboxRecorder is an httptest server that captures the last POSTed body.
type inboxRecorder struct {
	server *httptest.Server
	mu     sync.Mutex
	bodies []mapof.Any
	hits   int
}

// newInboxRecorder starts an inboxRecorder that answers every POST with 200 OK,
// and closes it when the test ends
func newInboxRecorder(t *testing.T) *inboxRecorder {
	t.Helper()
	recorder := &inboxRecorder{}
	recorder.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		parsed := mapof.NewAny()
		_ = json.Unmarshal(body, &parsed)

		recorder.mu.Lock()
		recorder.bodies = append(recorder.bodies, parsed)
		recorder.hits++
		recorder.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(recorder.server.Close)
	return recorder
}

// newSendingActor returns an Actor wired to deliver into the given recorder.
func newSendingActor(t *testing.T, recorder *inboxRecorder) Actor {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return NewActor("https://example.com/users/alice", privateKey,
		WithClient(mockClient{inboxURL: recorder.server.URL}),
		// These tests POST to loopback httptest servers that remote's SSRF guard
		// would otherwise block.
		WithAllowPrivateIPs(true))
}

// count returns the number of requests the recorder has received
func (r *inboxRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hits
}

// lastBody returns the most recent request body, or an empty map when nothing has arrived
func (r *inboxRecorder) lastBody() mapof.Any {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.bodies) == 0 {
		return mapof.NewAny()
	}
	return r.bodies[len(r.bodies)-1]
}
