package validator

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
)

// TestDeletedObject_NonDelete confirms the validator abstains (Unknown) for any
// activity that is not a Delete.
func TestDeletedObject_NonDelete(t *testing.T) {

	v := NewDeletedObject()

	activity := streams.NewDocument(map[string]any{
		vocab.PropertyType: vocab.ActivityTypeCreate,
	})

	assert.Equal(t, ResultUnknown, v.Validate(blankRequest(), &activity))
}

// TestDeletedObject_MissingObjectID confirms a Delete activity with no object ID
// is rejected as Invalid (there is nothing to confirm as deleted).
func TestDeletedObject_MissingObjectID(t *testing.T) {

	v := NewDeletedObject()

	activity := streams.NewDocument(map[string]any{
		vocab.PropertyType: vocab.ActivityTypeDelete,
		// No "object" property, so Object().ID() is empty.
	})

	assert.Equal(t, ResultInvalid, v.Validate(blankRequest(), &activity))
}

// TestDeletedObject_Responses confirms how each answer from the object's server decides the result.
func TestDeletedObject_Responses(t *testing.T) {

	// validateAgainst asks the validator about an object served with this status and body
	validateAgainst := func(status int, body string) Result {
		server, _ := serveDeleteTarget(t, status, body)
		activity := deleteActivity(server.URL)
		return DeletedObject{allowPrivateIPs: true}.Validate(blankRequest(), &activity)
	}

	// An object that is gone proves the Delete
	assert.Equal(t, ResultValid, validateAgainst(http.StatusNotFound, ""))
	assert.Equal(t, ResultValid, validateAgainst(http.StatusGone, ""))

	// An object that still exists disproves it
	assert.Equal(t, ResultInvalid, validateAgainst(http.StatusOK, `{"type":"Note"}`))

	// Any other failure proves nothing either way
	assert.Equal(t, ResultUnknown, validateAgainst(http.StatusInternalServerError, ""))
}

// TestDeletedObject_MaxResponseSize confirms the validator reads no more than streams.MaxResponseSize
func TestDeletedObject_MaxResponseSize(t *testing.T) {

	v := DeletedObject{allowPrivateIPs: true}

	// A document at the cap is read, so the object still exists
	server, _ := serveDeleteTarget(t, http.StatusOK, strings.Repeat("a", streams.MaxResponseSize))
	activity := deleteActivity(server.URL)
	assert.Equal(t, ResultInvalid, v.Validate(blankRequest(), &activity))

	// RULE: One byte over the cap fails the read, which proves nothing either way
	server, _ = serveDeleteTarget(t, http.StatusOK, strings.Repeat("a", streams.MaxResponseSize+1))
	activity = deleteActivity(server.URL)
	assert.Equal(t, ResultUnknown, v.Validate(blankRequest(), &activity))
}

// TestDeletedObject_GuardStaysOn confirms that NewDeletedObject refuses a private address, because
// the object's URL comes from an untrusted inbound activity.
func TestDeletedObject_GuardStaysOn(t *testing.T) {

	server, requests := serveDeleteTarget(t, http.StatusGone, "")
	activity := deleteActivity(server.URL)

	assert.Equal(t, ResultUnknown, NewDeletedObject().Validate(blankRequest(), &activity))
	assert.Equal(t, int32(0), requests.Load(), "the loopback server must never be contacted")
}

// serveDeleteTarget starts a loopback server that answers every request with one status and body,
// and counts the requests it receives.
func serveDeleteTarget(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {

	t.Helper()

	requests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server, requests
}

// deleteActivity returns a Delete activity whose object is the given URL
func deleteActivity(objectID string) streams.Document {
	return streams.NewDocument(map[string]any{
		vocab.PropertyType:   vocab.ActivityTypeDelete,
		vocab.PropertyObject: objectID,
	})
}
