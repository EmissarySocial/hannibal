package router

import (
	"errors"
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capture is a test context that records which route handler fired.
type capture struct {
	hit string
}

// handler returns a RouteHandler that records the given label into the capture
// context when invoked.
func handler(label string) RouteHandler[*capture] {
	return func(context *capture, _ streams.Document) error {
		context.hit = label
		return nil
	}
}

// activityDoc builds an in-memory activity document with the given activity type
// and a single embedded object of the given object type.
func activityDoc(activityType string, objectType string) streams.Document {
	return streams.NewDocument(map[string]any{
		vocab.PropertyType: activityType,
		vocab.PropertyObject: map[string]any{
			vocab.PropertyID:   "https://example.com/object",
			vocab.PropertyType: objectType,
		},
	})
}

// TestRouter_New confirms a new router starts with an empty route table.
func TestRouter_New(t *testing.T) {
	router := New[*capture]()
	require.NotNil(t, router.routes)
	assert.Empty(t, router.routes)
}

// TestRouter_Add confirms routes are keyed by "activityType/objectType".
func TestRouter_Add(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.ActivityTypeCreate, vocab.ObjectTypeNote, handler("create-note"))

	_, ok := router.routes[vocab.ActivityTypeCreate+"/"+vocab.ObjectTypeNote]
	assert.True(t, ok)
}

// TestRouter_Handle_ExactMatch confirms the most specific route is chosen.
func TestRouter_Handle_ExactMatch(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.ActivityTypeCreate, vocab.ObjectTypeNote, handler("exact"))
	router.Add(vocab.ActivityTypeCreate, vocab.Any, handler("activity-wildcard"))
	router.Add(vocab.Any, vocab.Any, handler("catch-all"))

	context := &capture{}
	err := router.Handle(context, activityDoc(vocab.ActivityTypeCreate, vocab.ObjectTypeNote))

	require.NoError(t, err)
	assert.Equal(t, "exact", context.hit, "the most specific route must win")
}

// TestRouter_Handle_ObjectWildcard confirms "*/object" matches when there is no
// exact activity/object route.
func TestRouter_Handle_ObjectWildcard(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.Any, vocab.ObjectTypeNote, handler("object-wildcard"))
	router.Add(vocab.Any, vocab.Any, handler("catch-all"))

	context := &capture{}
	err := router.Handle(context, activityDoc(vocab.ActivityTypeLike, vocab.ObjectTypeNote))

	require.NoError(t, err)
	assert.Equal(t, "object-wildcard", context.hit)
}

// TestRouter_Handle_ActivityWildcard confirms "activity/*" matches when no more
// specific route exists.
func TestRouter_Handle_ActivityWildcard(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.ActivityTypeCreate, vocab.Any, handler("activity-wildcard"))
	router.Add(vocab.Any, vocab.Any, handler("catch-all"))

	context := &capture{}
	err := router.Handle(context, activityDoc(vocab.ActivityTypeCreate, vocab.ObjectTypeImage))

	require.NoError(t, err)
	assert.Equal(t, "activity-wildcard", context.hit)
}

// TestRouter_Handle_CatchAll confirms "*/*" matches when nothing else does.
func TestRouter_Handle_CatchAll(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.Any, vocab.Any, handler("catch-all"))

	context := &capture{}
	err := router.Handle(context, activityDoc(vocab.ActivityTypeAnnounce, vocab.ObjectTypeVideo))

	require.NoError(t, err)
	assert.Equal(t, "catch-all", context.hit)
}

// TestRouter_Handle_NoMatch confirms an unmatched activity is a silent no-op
// (no error, no handler fired).
func TestRouter_Handle_NoMatch(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.ActivityTypeCreate, vocab.ObjectTypeNote, handler("create-note"))

	context := &capture{}
	err := router.Handle(context, activityDoc(vocab.ActivityTypeLike, vocab.ObjectTypeImage))

	require.NoError(t, err)
	assert.Equal(t, "", context.hit, "no handler should fire for an unmatched activity")
}

// TestRouter_Handle_PropagatesError confirms a handler's error is returned.
func TestRouter_Handle_PropagatesError(t *testing.T) {

	sentinel := errors.New("handler failed")

	router := New[*capture]()
	router.Add(vocab.ActivityTypeCreate, vocab.ObjectTypeNote,
		func(_ *capture, _ streams.Document) error {
			return sentinel
		})

	err := router.Handle(&capture{}, activityDoc(vocab.ActivityTypeCreate, vocab.ObjectTypeNote))
	assert.ErrorIs(t, err, sentinel)
}

// TestRouter_Handle_ImplicitCreate confirms that a bare object (not wrapped in an
// activity) is routed as an implicit "Create" activity.
func TestRouter_Handle_ImplicitCreate(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.ActivityTypeCreate, vocab.ObjectTypeNote, handler("implicit-create"))

	// A bare Note object -- no surrounding activity.
	bareObject := streams.NewDocument(map[string]any{
		vocab.AtContext:     vocab.ContextTypeActivityStreams,
		vocab.PropertyID:    "https://example.com/note/1",
		vocab.PropertyType:  vocab.ObjectTypeNote,
		vocab.PropertyActor: "https://example.com/actor",
	})

	context := &capture{}
	err := router.Handle(context, bareObject)

	require.NoError(t, err)
	assert.Equal(t, "implicit-create", context.hit, "a bare object must be routed as an implicit Create")
}
