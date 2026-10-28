package sender

import (
	"encoding/json"
	"iter"
	"sync"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSender_New_NilQueue confirms New tolerates a nil queue by creating an
// in-memory one (with a loud warning), so the returned Sender is usable.
func TestSender_New_NilQueue(t *testing.T) {

	sender := New(testLocator{}, nil)
	require.NotNil(t, sender.queue, "New must substitute an in-memory queue when none is provided")
}

// TestSender_Send confirms Send enqueues a single "send to all recipients" task.
func TestSender_Send(t *testing.T) {

	q, recorder := newRecordingQueue()
	sender := New(testLocator{}, q)

	activity := mapof.Any{
		vocab.PropertyActor: "https://test.actor.social",
		vocab.PropertyTo:    "https://test.actor.social/",
	}

	require.NoError(t, sender.Send(activity))
	assert.Equal(t, []string{OutboxSendToAllRecipients}, recorder.names())
}

// TestSender_Send_NilQueue confirms Send fails cleanly when the Sender has no
// queue (constructed directly, bypassing New).
func TestSender_Send_NilQueue(t *testing.T) {

	sender := Sender{locator: testLocator{}} // no queue
	err := sender.Send(mapof.Any{})
	require.Error(t, err)
}

// TestSendToAllRecipients confirms the activity is fanned out into one
// "send to single recipient" task per (deduplicated) recipient inbox.
func TestSendToAllRecipients(t *testing.T) {

	q, recorder := newRecordingQueue()
	sender := New(testLocator{}, q)

	activity := mapof.Any{
		vocab.PropertyActor: "https://test.actor.social",
		// "followers" resolves to three distinct inbox URLs via testLocator.
		vocab.PropertyTo: "https://test.actor.social/followers",
	}

	result := sender.SendToAllRecipients(activity)
	require.Equal(t, queue.ResultStatusSuccess, result.Status)

	// One task per follower inbox.
	names := recorder.names()
	assert.Len(t, names, 3)
	for _, name := range names {
		assert.Equal(t, OutboxSendToSingleRecipient, name)
	}
}

// TestSendToAllRecipients_Dedup confirms duplicate recipient inboxes collapse to
// a single send task.
func TestSendToAllRecipients_Dedup(t *testing.T) {

	q, recorder := newRecordingQueue()
	sender := New(testLocator{}, q)

	// "to" and "cc" both point at the same single recipient, which resolves to
	// the same inbox URL -- it must only be sent once.
	activity := mapof.Any{
		vocab.PropertyActor: "https://test.actor.social",
		vocab.PropertyTo:    "https://test.actor.social/",
		vocab.PropertyCC:    "https://test.actor.social/",
	}

	result := sender.SendToAllRecipients(activity)
	require.Equal(t, queue.ResultStatusSuccess, result.Status)
	assert.Len(t, recorder.names(), 1, "duplicate inbox URLs must be deduplicated")
}

// TestSendToAllRecipients_StripsBccBto confirms the BCC and BTo fields are
// removed from the activity before it is enqueued for delivery (so recipients
// never see the blind addressing).
func TestSendToAllRecipients_StripsBccBto(t *testing.T) {

	q, recorder := newRecordingQueue()
	sender := New(testLocator{}, q)

	activity := mapof.Any{
		vocab.PropertyActor: "https://test.actor.social",
		vocab.PropertyTo:    "https://test.actor.social/",
		vocab.PropertyBCC:   "https://secret.example.com/",
		vocab.PropertyBTo:   "https://secret.example.com/",
	}

	result := sender.SendToAllRecipients(activity)
	require.Equal(t, queue.ResultStatusSuccess, result.Status)

	// Inspect the enqueued body -- BCC/BTo must be gone.
	require.NotEmpty(t, recorder.tasks)
	sentActivity := mapof.NewAny()
	require.NoError(t, json.Unmarshal([]byte(recorder.tasks[0].Arguments.GetString("body")), &sentActivity))
	assert.Equal(t, "https://test.actor.social", sentActivity.GetString(vocab.PropertyActor))
	assert.NotContains(t, sentActivity, vocab.PropertyBCC)
	assert.NotContains(t, sentActivity, vocab.PropertyBTo)
}

// TestSendToAllRecipients_Body confirms every recipient's task carries the same serialized body, and no activity map.
func TestSendToAllRecipients_Body(t *testing.T) {

	q, recorder := newRecordingQueue()
	sender := New(testLocator{}, q)

	activity := mapof.Any{
		vocab.PropertyActor: "https://test.actor.social",
		vocab.PropertyType:  vocab.ActivityTypeCreate,
		// "followers" resolves to three distinct inbox URLs via testLocator.
		vocab.PropertyTo: "https://test.actor.social/followers",
	}

	result := sender.SendToAllRecipients(activity)
	require.Equal(t, queue.ResultStatusSuccess, result.Status)
	require.Len(t, recorder.tasks, 3)

	// Every task carries the same bytes, which decode back to the activity
	expected, err := json.Marshal(activity)
	require.NoError(t, err)

	for _, task := range recorder.tasks {
		assert.Equal(t, string(expected), task.Arguments.GetString("body"))
		assert.NotContains(t, task.Arguments, "activity")
	}
}

// TestSendToAllRecipients_ActorNotFound confirms an unknown sending actor yields
// a Failure result (it cannot be retried).
func TestSendToAllRecipients_ActorNotFound(t *testing.T) {

	q, _ := newRecordingQueue()
	sender := New(testLocator{}, q)

	activity := mapof.Any{
		vocab.PropertyActor: "https://unknown.example.com/actor",
		vocab.PropertyTo:    "https://test.actor.social/",
	}

	result := sender.SendToAllRecipients(activity)
	assert.Equal(t, queue.ResultStatusFailure, result.Status)
}

// TestSendToAllRecipients_RecipientError confirms a failure to resolve recipients
// is surfaced as an Error (retriable).
func TestSendToAllRecipients_RecipientError(t *testing.T) {

	q, _ := newRecordingQueue()
	sender := New(erroringLocator{}, q)

	activity := mapof.Any{
		vocab.PropertyActor: "https://test.actor.social",
		vocab.PropertyTo:    "https://test.actor.social/",
	}

	result := sender.SendToAllRecipients(activity)
	assert.Equal(t, queue.ResultStatusError, result.Status)
}

// TestConsumer confirms the queue Consumer routes task names to the matching
// Sender method, and ignores unknown task names.
func TestConsumer(t *testing.T) {

	q, _ := newRecordingQueue()
	sender := New(testLocator{}, q)
	consumer := Consumer(sender)

	// A known "all recipients" task is handled (success for a valid actor).
	allResult := consumer.Run(queue.Task{
		Name: OutboxSendToAllRecipients,
		Arguments: mapof.Any{
			vocab.PropertyActor: "https://test.actor.social",
			vocab.PropertyTo:    "https://test.actor.social/",
		},
	})
	assert.Equal(t, queue.ResultStatusSuccess, allResult.Status)

	// A "single recipient" task is also routed to the sender. With an unknown
	// actor it Fails, but the point here is that the Consumer dispatched it (not
	// Ignored) -- proving the route is wired.
	singleResult := consumer.Run(queue.Task{
		Name: OutboxSendToSingleRecipient,
		Arguments: mapof.Any{
			"actor":    "https://unknown.example.com/actor",
			"inbox":    "https://example.com/inbox",
			"activity": mapof.Any{},
		},
	})
	assert.NotEqual(t, queue.ResultStatusIgnored, singleResult.Status)

	// An unknown task name is ignored (left for other consumers).
	ignored := consumer.Run(queue.Task{Name: "Some:OtherTask", Arguments: mapof.Any{}})
	assert.Equal(t, queue.ResultStatusIgnored, ignored.Status)

	// The lifecycle hooks are required by the interface, and this consumer has nothing to say.
	// OnPublish returning nil is what lets an outbound activity onto the queue at all -- a
	// non-nil error here would reject every send before it was ever stored.
	assert.Nil(t, consumer.OnPublish(&queue.Task{}))
	assert.Nil(t, consumer.OnSuccess(queue.Task{}))
	assert.Nil(t, consumer.OnError(queue.Task{}, nil))
	assert.Nil(t, consumer.OnFailure(queue.Task{}, nil))
}

// taskRecorder captures every task published to a queue, via the queue's
// PreProcessor hook. This lets tests assert what was enqueued without running
// any worker goroutines.
type taskRecorder struct {
	mu    sync.Mutex
	tasks []queue.Task
}

// newRecordingQueue builds a queue whose published tasks are captured by the
// returned recorder. A generous buffer keeps Publish from blocking.
func newRecordingQueue() (*queue.Queue, *taskRecorder) {
	recorder := &taskRecorder{}
	q := queue.New(
		queue.WithPreProcessor(recorder.preProcessor),
		queue.WithBufferSize(128),
	)
	return q, recorder
}

// preProcessor records a copy of each task the queue publishes
func (r *taskRecorder) preProcessor(task *queue.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = append(r.tasks, *task)
	return nil
}

// names returns the name of every recorded task, in the order published
func (r *taskRecorder) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]string, 0, len(r.tasks))
	for _, task := range r.tasks {
		result = append(result, task.Name)
	}
	return result
}

// erroringLocator finds the test actor but fails to resolve any recipient.
type erroringLocator struct{}

// Actor returns the test actor for its own ID, and NotFound for any other ID
func (erroringLocator) Actor(id string) (Actor, error) {
	if id == "https://test.actor.social" {
		return testActor{}, nil
	}
	return nil, derp.NotFound("erroringLocator.Actor", "unknown", id)
}

// Recipient always fails
func (erroringLocator) Recipient(_ string) (iter.Seq[string], error) {
	return nil, derp.Internal("erroringLocator.Recipient", "resolution failed")
}
