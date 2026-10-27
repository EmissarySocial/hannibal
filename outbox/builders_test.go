package outbox

import (
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addressedDoc builds a document of the given type, addressed to a remote
// recipient (so the builder's Send fans out to the inbox recorder).
func addressedDoc(id string, objectType string) streams.Document {
	return streams.NewDocument(map[string]any{
		vocab.PropertyID:   id,
		vocab.PropertyType: objectType,
		vocab.PropertyTo:   "https://remote.example.com/users/bob",
	})
}

// assertDelivered runs the builder, then asserts exactly one activity was
// delivered with the expected type and the correct actor URL. Each builder is a
// near-identical message constructor; this guards every one against the kind of
// copy/paste slip that broke SendUndo.
func assertDelivered(t *testing.T, build func(actor *Actor, recorder *inboxRecorder), expectedType string) {
	t.Helper()

	recorder := newInboxRecorder(t)
	actor := newSendingActor(t, recorder)

	build(&actor, recorder)

	require.Equal(t, 1, recorder.count(), "%s must be delivered", expectedType)
	body := recorder.lastBody()
	assert.Equal(t, expectedType, body.GetString(vocab.PropertyType))
	assert.Equal(t, "https://example.com/users/alice", body.GetString(vocab.PropertyActor),
		"the actor field must be the actor URL string")
}

// TestSendBuilders exercises every remaining SendXxx builder, confirming each
// produces a deliverable activity of the correct type with the correct actor.
func TestSendBuilders(t *testing.T) {

	object := addressedDoc("https://remote.example.com/notes/1", vocab.ObjectTypeNote)

	t.Run("SendCreate", func(t *testing.T) {
		assertDelivered(t, func(a *Actor, _ *inboxRecorder) {
			a.SendCreate(object)
		}, vocab.ActivityTypeCreate)
	})

	t.Run("SendUpdate", func(t *testing.T) {
		assertDelivered(t, func(a *Actor, _ *inboxRecorder) {
			a.SendUpdate(object)
		}, vocab.ActivityTypeUpdate)
	})

	t.Run("SendDelete", func(t *testing.T) {
		// Delete reads document.Object(), so wrap the note in a delete activity.
		deleteActivity := streams.NewDocument(map[string]any{
			vocab.PropertyTo: "https://remote.example.com/users/bob",
			vocab.PropertyObject: map[string]any{
				vocab.PropertyID:   "https://remote.example.com/notes/1",
				vocab.PropertyType: vocab.ObjectTypeNote,
			},
		})
		assertDelivered(t, func(a *Actor, _ *inboxRecorder) {
			a.SendDelete(deleteActivity)
		}, vocab.ActivityTypeDelete)
	})

	t.Run("SendLike", func(t *testing.T) {
		assertDelivered(t, func(a *Actor, _ *inboxRecorder) {
			a.SendLike("https://example.com/activities/like-1", object)
		}, vocab.ActivityTypeLike)
	})

	t.Run("SendDislike", func(t *testing.T) {
		assertDelivered(t, func(a *Actor, _ *inboxRecorder) {
			a.SendDislike("https://example.com/activities/dislike-1", object)
		}, vocab.ActivityTypeDislike)
	})

	t.Run("SendAnnounce", func(t *testing.T) {
		assertDelivered(t, func(a *Actor, _ *inboxRecorder) {
			a.SendAnnounce("https://example.com/activities/announce-1", object)
		}, vocab.ActivityTypeAnnounce)
	})

	t.Run("SendAccept", func(t *testing.T) {
		// Accept addresses the actor of the accepted activity.
		followActivity := streams.NewDocument(map[string]any{
			vocab.PropertyType: vocab.ActivityTypeFollow,
			vocab.PropertyActor: map[string]any{
				vocab.PropertyID: "https://remote.example.com/users/bob",
			},
		})
		assertDelivered(t, func(a *Actor, _ *inboxRecorder) {
			a.SendAccept("https://example.com/activities/accept-1", followActivity)
		}, vocab.ActivityTypeAccept)
	})
}
