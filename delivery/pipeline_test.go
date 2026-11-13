package delivery

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/remote"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/require"
)

/******************************************
 * Test Doubles
 ******************************************/

// testKey is one RSA key shared by every test, because generating one is slow
var testKey = sync.OnceValue(func() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
})

// testSigner is a Signer for the local test actor
type testSigner struct{}

// ActorID returns the local test actor
func (testSigner) ActorID() string {
	return testActor
}

// PrivateKey returns the shared test key
func (testSigner) PrivateKey() (string, crypto.PrivateKey) {
	return testActor + "#main-key", testKey()
}

// testSigners returns the signer for the local test actor, and NotFound for anyone else
func testSigners(actorID string) (Signer, error) {

	if actorID != testActor {
		return nil, derp.NotFound("test", "Actor not found", actorID)
	}

	return testSigner{}, nil
}

// taskRecorder records every task that a queue is asked to publish
type taskRecorder struct {
	mutex sync.Mutex
	tasks []queue.Task
}

// preProcessor records a copy of the task
func (recorder *taskRecorder) preProcessor(task *queue.Task) error {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	recorder.tasks = append(recorder.tasks, *task)
	return nil
}

// recipients returns the recipient of every recorded task, in the order published
func (recorder *taskRecorder) recipients() []string {

	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()

	result := make([]string, 0, len(recorder.tasks))

	for _, task := range recorder.tasks {
		result = append(result, task.Arguments.GetString("recipient"))
	}

	return result
}

// newRecordingQueue returns a queue with no workers, and the recorder that sees what it is given
func newRecordingQueue() (*queue.Queue, *taskRecorder) {
	recorder := &taskRecorder{}
	return queue.New(queue.WithPreProcessor(recorder.preProcessor), queue.WithBufferSize(256)), recorder
}

// documentClient is a streams.Client that serves documents from a map, and counts its loads
type documentClient struct {
	documents map[string]map[string]any
	errors    map[string]error
	loads     []string
}

// SetRootClient implements streams.Client
func (client *documentClient) SetRootClient(streams.Client) {}

// Load returns the document for a URL, the error set for it, or NotFound
func (client *documentClient) Load(url string, _ ...any) (streams.Document, error) {

	client.loads = append(client.loads, url)

	if err := client.errors[url]; err != nil {
		return streams.NilDocument(), err
	}

	document, found := client.documents[url]

	if !found {
		return streams.NilDocument(), derp.NotFound("test", "Document not found", url)
	}

	return streams.NewDocument(document, streams.WithClient(client)), nil
}

// Save implements streams.Client
func (client *documentClient) Save(streams.Document) error { return nil }

// Delete implements streams.Client
func (client *documentClient) Delete(string) error { return nil }

// inboxServer is a remote server that records what is delivered to it
type inboxServer struct {
	*httptest.Server
	mutex    sync.Mutex
	status   int
	header   http.Header
	requests []*http.Request
	bodies   []string
}

// newInboxServer returns a running server that answers every POST with 202
func newInboxServer(t *testing.T) *inboxServer {

	t.Helper()

	result := &inboxServer{status: http.StatusAccepted, header: http.Header{}}

	result.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {

		body, _ := io.ReadAll(request.Body)

		result.mutex.Lock()
		result.requests = append(result.requests, request)
		result.bodies = append(result.bodies, string(body))
		status := result.status
		result.mutex.Unlock()

		for name, values := range result.header {
			writer.Header()[name] = values
		}

		writer.WriteHeader(status)
	}))

	t.Cleanup(result.Close)
	return result
}

// actor returns the URL of an actor on this server, and its actor document
func (server *inboxServer) actor(name string) (string, map[string]any) {

	actorURL := server.URL + "/@" + name

	return actorURL, map[string]any{
		"id":        actorURL,
		"type":      "Person",
		"inbox":     actorURL + "/inbox",
		"followers": actorURL + "/followers",
		"endpoints": map[string]any{"sharedInbox": server.URL + "/shared-inbox"},
	}
}

// count returns the number of requests that the server has received
func (server *inboxServer) count() int {
	server.mutex.Lock()
	defer server.mutex.Unlock()
	return len(server.requests)
}

// testHostList is a HostList that holds the hosts it names
type testHostList []string

// Contains returns TRUE if the host is in the list
func (list testHostList) Contains(host string) bool {
	return slices.Contains(list, host)
}

// testRefusal is an error that carries a status code and a wait, as a circuit breaker's does
type testRefusal struct {
	code int
	wait time.Duration
}

// Error returns the error message
func (refusal testRefusal) Error() string {
	return "refused by an option"
}

// GetErrorCode returns the status code, which derp.ErrorCode reads
func (refusal testRefusal) GetErrorCode() int {
	return refusal.code
}

// GetRetryAfter returns the wait, which derp.RetryAfter reads
func (refusal testRefusal) GetRetryAfter() time.Duration {
	return refusal.wait
}

// refuseWith returns the remote options of an application that refuses every request with one error
func refuseWith(refusal error, calls *int) func() []remote.Option {
	return func() []remote.Option {
		*calls++
		return []remote.Option{{
			BeforeRequest: func(*remote.Transaction) error {
				return refusal
			},
		}}
	}
}

/******************************************
 * FanOut
 ******************************************/

// TestFanOut verifies the whole fan-out: collections expand, duplicates and excluded
// recipients go, email is routed apart, and each task carries one stripped body
func TestFanOut(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()

	collections := &testCollections{members: map[string][]string{
		testFollowers: {testAlice, testBob, "mailto:dana@example.org", "https://paused.example/@erin"},
		testCircle:    {testAlice, testCarol},
	}}

	type email struct {
		activityID string
		address    string
		activity   map[string]any
	}

	emails := make([]email, 0)
	filtered := make([]string, 0)

	pipeline := New(taskQueue, &documentClient{}, testSigners,
		WithBlockedHosts(func() (HostList, error) { return testHostList{"paused.example"}, nil }),
		WithCollections(collections),
		WithRelays(func(map[string]any) iter.Seq[string] { return slices.Values([]string{testRelay}) }),
		WithFilter(func(actorID string) (func(string) bool, error) {
			require.Equal(t, testActor, actorID)
			return func(recipientURL string) bool {
				filtered = append(filtered, recipientURL)
				return recipientURL != testBob
			}, nil
		}),
		WithEmail(func(activityID string, address string, activity map[string]any) error {
			emails = append(emails, email{activityID: activityID, address: address, activity: activity})
			return nil
		}),
	)

	activity := mapof.Any{
		"id":    testActor + "/pub/outbox/1",
		"type":  "Create",
		"actor": testActor,
		"to":    []any{vocab.NamespacePublic, testCircle},
		"cc":    []any{testFollowers},
		"bcc":   []any{"https://blind.example/@frank"},
		"object": map[string]any{
			"type":    "Note",
			"content": "hello",
			"bto":     []any{"https://blind.example/@frank"},
		},
	}

	result := pipeline.FanOut(mapof.Any{"activity": activity})
	require.Equal(t, queue.ResultStatusSuccess, result.Status)

	// One task per reachable, allowed, unique actor. Bob is excluded, Erin's host is paused,
	// and Alice is in two collections.
	require.Equal(t, []string{testRelay, testAlice, testCarol, "https://blind.example/@frank"}, recorder.recipients())
	require.NotContains(t, filtered, "https://paused.example/@erin", "a blocked host is dropped before the filter runs")

	// Every task carries the same body, with no blind recipients at any level
	body := recorder.tasks[0].Arguments.GetString("body")
	require.NotContains(t, body, "blind.example")
	require.Contains(t, body, `"content":"hello"`)

	hash := sha256.Sum256([]byte(body))
	encoded := base64.StdEncoding.EncodeToString(hash[:])

	for _, task := range recorder.tasks {
		require.Equal(t, TaskSend, task.Name)
		require.Equal(t, testActor, task.Arguments.GetString("actor"))
		require.Equal(t, body, task.Arguments.GetString("body"))
		require.Equal(t, "SHA-256="+encoded, task.Arguments.GetString("digest"))
		require.Equal(t, "sha-256=:"+encoded+":", task.Arguments.GetString("contentDigest"))
		require.Equal(t, testActor+"/pub/outbox/1 "+task.Arguments.GetString("recipient"), task.Signature)
		require.Equal(t, 17, task.RetryMax)
		require.Equal(t, int64(86400), task.BackoffMax)
	}

	// The email follower is handed to the application, with the stripped activity
	require.Len(t, emails, 1)
	require.Equal(t, "mailto:dana@example.org", emails[0].address)
	require.Equal(t, testActor+"/pub/outbox/1", emails[0].activityID)
	require.NotContains(t, emails[0].activity, "bcc")

	// The caller's activity keeps its blind recipients
	require.Contains(t, activity, "bcc")
}

// TestFanOut_UnlistedReachesNoRelay verifies that an activity with Public only in `cc` goes to no relay
func TestFanOut_UnlistedReachesNoRelay(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()

	pipeline := New(taskQueue, &documentClient{}, testSigners,
		WithRelays(func(map[string]any) iter.Seq[string] { return slices.Values([]string{testRelay}) }),
	)

	activity := mapof.Any{"id": testActor + "/1", "actor": testActor, "to": []any{testAlice}, "cc": []any{vocab.NamespacePublic}}

	require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": activity}).Status)
	require.Equal(t, []string{testAlice}, recorder.recipients())
}

// TestFanOut_RetryQueuesTheSameSignatures verifies that a second run of a fan-out names every
// task as the first did, so that the queue can refuse the duplicates
func TestFanOut_RetryQueuesTheSameSignatures(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()
	pipeline := New(taskQueue, &documentClient{}, testSigners)
	activity := mapof.Any{"id": testActor + "/1", "actor": testActor, "to": []any{testAlice, testBob}}

	require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": activity}).Status)
	require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": activity}).Status)

	require.Len(t, recorder.tasks, 4)
	require.Equal(t, recorder.tasks[0].Signature, recorder.tasks[2].Signature)
	require.Equal(t, recorder.tasks[1].Signature, recorder.tasks[3].Signature)
	require.NotEqual(t, recorder.tasks[0].Signature, recorder.tasks[1].Signature)
}

// TestFanOut_CollectionErrorQueuesNothing verifies that a collection that cannot be read fails
// the fan-out before any task is queued, so that the retry reaches everyone
func TestFanOut_CollectionErrorQueuesNothing(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()
	collections := &testCollections{errors: map[string]error{testFollowers: errors.New("database is down")}}
	pipeline := New(taskQueue, &documentClient{}, testSigners, WithCollections(collections))

	activity := mapof.Any{"id": testActor + "/1", "actor": testActor, "to": []any{testAlice, testFollowers}}

	result := pipeline.FanOut(mapof.Any{"activity": activity})
	require.Equal(t, queue.ResultStatusError, result.Status, "the fan-out is retried")
	require.Empty(t, recorder.tasks)
}

// TestFanOut_FilterErrorQueuesNothing verifies that a filter that cannot be built fails the fan-out
func TestFanOut_FilterErrorQueuesNothing(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()

	pipeline := New(taskQueue, &documentClient{}, testSigners,
		WithFilter(func(string) (func(string) bool, error) { return nil, errors.New("rules are unavailable") }),
	)

	activity := mapof.Any{"id": testActor + "/1", "actor": testActor, "to": []any{testAlice}}

	require.Equal(t, queue.ResultStatusError, pipeline.FanOut(mapof.Any{"activity": activity}).Status)
	require.Empty(t, recorder.tasks)
}

// TestFanOut_UnknownActor verifies that an activity from an actor the application does not own fails for good
func TestFanOut_UnknownActor(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()
	pipeline := New(taskQueue, &documentClient{}, testSigners)

	activity := mapof.Any{"id": "https://evil.example/1", "actor": "https://evil.example/@mallory", "to": []any{testAlice}}

	require.Equal(t, queue.ResultStatusFailure, pipeline.FanOut(mapof.Any{"activity": activity}).Status)
	require.Equal(t, queue.ResultStatusFailure, pipeline.FanOut(mapof.Any{}).Status, "a task with no activity names no actor")
	require.Empty(t, recorder.tasks)
}

// TestFanOut_Proof verifies that the proof is attached after the strip, and that a failed
// proof still sends the activity
func TestFanOut_Proof(t *testing.T) {

	activity := mapof.Any{"id": testActor + "/1", "actor": testActor, "to": []any{testAlice}, "bcc": []any{testBob}}

	t.Run("attached", func(t *testing.T) {

		taskQueue, recorder := newRecordingQueue()

		pipeline := New(taskQueue, &documentClient{}, testSigners,
			WithProof(func(actorID string, stripped map[string]any) (map[string]any, error) {
				require.Equal(t, testActor, actorID)
				require.NotContains(t, stripped, "bcc", "the proof must be made over the stripped activity")
				stripped["proof"] = map[string]any{"type": "DataIntegrityProof"}
				return stripped, nil
			}),
		)

		require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": activity}).Status)
		require.Contains(t, recorder.tasks[0].Arguments.GetString("body"), `"DataIntegrityProof"`)
	})

	t.Run("failed", func(t *testing.T) {

		taskQueue, recorder := newRecordingQueue()

		pipeline := New(taskQueue, &documentClient{}, testSigners,
			WithProof(func(string, map[string]any) (map[string]any, error) {
				return nil, errors.New("no key")
			}),
		)

		require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": activity}).Status)
		require.Len(t, recorder.tasks, 2)
		require.NotContains(t, recorder.tasks[0].Arguments.GetString("body"), "proof")
	})
}

// TestFanOut_ActivityWithoutID verifies that an activity with no id is identified by its content
func TestFanOut_ActivityWithoutID(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()
	pipeline := New(taskQueue, &documentClient{}, testSigners)

	require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": mapof.Any{"actor": testActor, "to": testAlice}}).Status)
	require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": mapof.Any{"actor": testActor, "to": testAlice, "type": "Other"}}).Status)

	require.Len(t, recorder.tasks, 2)
	require.Regexp(t, `^sha256:[0-9a-f]{64} `, recorder.tasks[0].Signature)
	require.NotEqual(t, recorder.tasks[0].Signature, recorder.tasks[1].Signature)
}

// TestFanOut_DropsWhatHasNoInbox verifies that email with no handler, and a recipient that is
// not a URL, are dropped without failing the fan-out
func TestFanOut_DropsWhatHasNoInbox(t *testing.T) {

	taskQueue, recorder := newRecordingQueue()
	pipeline := New(taskQueue, &documentClient{}, testSigners)

	activity := mapof.Any{"id": testActor + "/1", "actor": testActor, "to": []any{"mailto:dana@example.org", "not a url", testAlice}}

	require.Equal(t, queue.ResultStatusSuccess, pipeline.FanOut(mapof.Any{"activity": activity}).Status)
	require.Equal(t, []string{testAlice}, recorder.recipients())
}

/******************************************
 * Send
 ******************************************/

// sendTest holds everything one Send test needs: a remote server, a client that knows its
// actors, and the Pipeline under test
type sendTest struct {
	server   *inboxServer
	client   *documentClient
	pipeline Pipeline
	alice    string
}

// newSendTest returns a Pipeline that can deliver to an actor named alice on a local test server
func newSendTest(t *testing.T, options ...PipelineOption) sendTest {

	t.Helper()

	server := newInboxServer(t)
	alice, aliceDocument := server.actor("alice")
	client := &documentClient{documents: map[string]map[string]any{alice: aliceDocument}, errors: map[string]error{}}

	options = append([]PipelineOption{WithAllowPrivateIPs(true)}, options...)

	return sendTest{
		server:   server,
		client:   client,
		pipeline: New(nil, client, testSigners, options...),
		alice:    alice,
	}
}

// args returns the arguments of a send task for the given recipient
func (test sendTest) args(recipientURL string) mapof.Any {
	return mapof.Any{
		"actor":     testActor,
		"recipient": recipientURL,
		"body":      `{"id":"https://example.com/@author/1","type":"Create","actor":"https://example.com/@author"}`,
	}
}

// TestSend_Delivers verifies that the body reaches the actor's own inbox as it was queued, in
// a signed request
func TestSend_Delivers(t *testing.T) {

	test := newSendTest(t)

	result := test.pipeline.Send(test.args(test.alice))
	require.Equal(t, queue.ResultStatusSuccess, result.Status)

	require.Equal(t, 1, test.server.count())
	request := test.server.requests[0]
	require.Equal(t, "/@alice/inbox", request.URL.Path, "the personal inbox, never the shared one")
	require.Equal(t, test.args(test.alice).GetString("body"), test.server.bodies[0])
	require.Equal(t, vocab.ContentTypeActivityPub, request.Header.Get("Content-Type"))

	signature := request.Header.Get("Signature")
	require.Contains(t, signature, `keyId="`+testActor+`#main-key"`)
	require.Contains(t, signature, `headers="(request-target) host date digest"`)
	require.NotEmpty(t, request.Header.Get("Digest"))
}

// TestSend_InboxOnAnotherHost verifies that an actor whose inbox is on another host gets nothing
func TestSend_InboxOnAnotherHost(t *testing.T) {

	test := newSendTest(t)
	victim := newInboxServer(t)

	mallory, document := test.server.actor("mallory")
	document["inbox"] = victim.URL + "/inbox"
	test.client.documents[mallory] = document

	result := test.pipeline.Send(test.args(mallory))
	require.Equal(t, queue.ResultStatusFailure, result.Status, "the delivery is dropped, not retried")
	require.Zero(t, victim.count())
	require.Zero(t, test.server.count())
}

// TestSend_NoInbox verifies that an actor with no inbox fails for good
func TestSend_NoInbox(t *testing.T) {

	test := newSendTest(t)

	silent, document := test.server.actor("silent")
	delete(document, "inbox")
	test.client.documents[silent] = document

	require.Equal(t, queue.ResultStatusFailure, test.pipeline.Send(test.args(silent)).Status)
	require.Zero(t, test.server.count())
}

// TestSend_Outcomes verifies what each answer from the remote server does to the task
func TestSend_Outcomes(t *testing.T) {

	for name, status := range map[string]int{"server error": http.StatusBadGateway, "request timeout": http.StatusRequestTimeout, "gateway timeout": http.StatusGatewayTimeout} {

		t.Run(name+" is retried", func(t *testing.T) {

			test := newSendTest(t)
			test.server.status = status

			require.Equal(t, queue.ResultStatusError, test.pipeline.Send(test.args(test.alice)).Status)
		})
	}

	t.Run("rate limit with no wait named uses derp's default of a minute", func(t *testing.T) {

		test := newSendTest(t)
		test.server.status = http.StatusTooManyRequests

		result := test.pipeline.Send(test.args(test.alice))
		require.Equal(t, queue.ResultStatusRequeue, result.Status)
		require.Equal(t, time.Minute, result.Delay)
	})

	t.Run("rate limit reschedules for as long as the host asked", func(t *testing.T) {

		test := newSendTest(t)
		test.server.status = http.StatusTooManyRequests
		test.server.header.Set("Retry-After", "600")

		result := test.pipeline.Send(test.args(test.alice))
		require.Equal(t, queue.ResultStatusRequeue, result.Status)
		require.Equal(t, 10*time.Minute, result.Delay)
	})

	t.Run("rate limit never reschedules for longer than an hour", func(t *testing.T) {

		test := newSendTest(t)
		test.server.status = http.StatusTooManyRequests
		test.server.header.Set("Retry-After", "86400")

		result := test.pipeline.Send(test.args(test.alice))
		require.Equal(t, queue.ResultStatusRequeue, result.Status)
		require.Equal(t, time.Hour, result.Delay)
	})

	for name, status := range map[string]int{"unauthorized": http.StatusUnauthorized, "forbidden": http.StatusForbidden, "bad request": http.StatusBadRequest, "gone": http.StatusGone, "not found": http.StatusNotFound} {

		t.Run(name+" fails for good", func(t *testing.T) {

			test := newSendTest(t)
			test.server.status = status

			require.Equal(t, queue.ResultStatusFailure, test.pipeline.Send(test.args(test.alice)).Status)
		})
	}
}

// TestSend_RefusedByAnOption verifies what the task does when one of the application's options
// refuses the request before it is sent, as a circuit breaker does
func TestSend_RefusedByAnOption(t *testing.T) {

	t.Run("a host that gets nothing is skipped quietly", func(t *testing.T) {

		calls := 0
		test := newSendTest(t, WithRemoteOptions(refuseWith(testRefusal{code: http.StatusLocked}, &calls)))

		require.Equal(t, queue.ResultStatusSuccess, test.pipeline.Send(test.args(test.alice)).Status)
		require.Zero(t, test.server.count())
		require.Equal(t, 1, calls)
	})

	t.Run("a rate-limited host is rescheduled for the wait it is given", func(t *testing.T) {

		calls := 0
		test := newSendTest(t, WithRemoteOptions(refuseWith(testRefusal{code: http.StatusTooManyRequests, wait: 5 * time.Minute}, &calls)))

		result := test.pipeline.Send(test.args(test.alice))
		require.Equal(t, queue.ResultStatusRequeue, result.Status)
		require.Equal(t, 5*time.Minute, result.Delay)
		require.Zero(t, test.server.count())
	})

	t.Run("an unhealthy host is retried later", func(t *testing.T) {

		calls := 0
		test := newSendTest(t, WithRemoteOptions(refuseWith(testRefusal{code: http.StatusServiceUnavailable, wait: time.Minute}, &calls)))

		require.Equal(t, queue.ResultStatusError, test.pipeline.Send(test.args(test.alice)).Status)
		require.Zero(t, test.server.count())
	})

	t.Run("the options are built again for every delivery", func(t *testing.T) {

		calls := 0
		test := newSendTest(t, WithRemoteOptions(refuseWith(testRefusal{code: http.StatusLocked}, &calls)))

		for range 3 {
			test.pipeline.Send(test.args(test.alice))
		}

		require.Equal(t, 3, calls)
	})
}

// TestRequeueDelay verifies that a rescheduled delivery never waits less than a second or more than an hour
func TestRequeueDelay(t *testing.T) {
	require.Equal(t, time.Second, requeueDelay(0))
	require.Equal(t, time.Second, requeueDelay(-time.Minute))
	require.Equal(t, 90*time.Second, requeueDelay(90*time.Second))
	require.Equal(t, time.Hour, requeueDelay(48*time.Hour))
}

// TestSend_RecipientCannotBeLoaded verifies that a recipient who is gone fails for good, and
// that any other load error is retried
func TestSend_RecipientCannotBeLoaded(t *testing.T) {

	test := newSendTest(t)

	require.Equal(t, queue.ResultStatusFailure, test.pipeline.Send(test.args(test.server.URL+"/@nobody")).Status)

	test.client.errors[test.alice] = derp.Internal("test", "timeout")
	require.Equal(t, queue.ResultStatusError, test.pipeline.Send(test.args(test.alice)).Status)

	test.client.errors[test.alice] = derp.Internal("test", "gone", derp.WithCode(http.StatusGone))
	require.Equal(t, queue.ResultStatusFailure, test.pipeline.Send(test.args(test.alice)).Status)

	require.Zero(t, test.server.count())
}

// TestSend_RefusesBadTasks verifies that a task with no body, or from an actor the application
// does not own, fails for good
func TestSend_RefusesBadTasks(t *testing.T) {

	test := newSendTest(t)

	noBody := test.args(test.alice)
	delete(noBody, "body")
	require.Equal(t, queue.ResultStatusFailure, test.pipeline.Send(noBody).Status)

	stranger := test.args(test.alice)
	stranger["actor"] = "https://evil.example/@mallory"
	require.Equal(t, queue.ResultStatusFailure, test.pipeline.Send(stranger).Status)

	require.Zero(t, test.server.count())
}

// replyArgs returns a send task whose activity replies to a post by alice and names her followers
func (test sendTest) replyArgs(t *testing.T, recipientURL string) mapof.Any {

	t.Helper()

	parent := test.server.URL + "/notes/1"
	test.client.documents[parent] = map[string]any{"id": parent, "type": "Note", "attributedTo": test.alice}

	body, err := json.Marshal(map[string]any{
		"id":    testActor + "/1",
		"type":  "Create",
		"actor": testActor,
		"cc":    []any{recipientURL},
		"object": map[string]any{
			"type":      "Note",
			"inReplyTo": parent,
		},
	})
	require.NoError(t, err)

	return mapof.Any{"actor": testActor, "recipient": recipientURL, "body": string(body)}
}

// TestSend_RemoteCollection verifies that a remote collection is not expanded: the delivery
// goes to its owner, in the same task
func TestSend_RemoteCollection(t *testing.T) {

	t.Run("delivered to the owner", func(t *testing.T) {

		test := newSendTest(t)
		followers := test.alice + "/followers"
		test.client.documents[followers] = map[string]any{"id": followers, "type": "OrderedCollection", "totalItems": 5000}

		require.Equal(t, queue.ResultStatusSuccess, test.pipeline.Send(test.replyArgs(t, followers)).Status)
		require.Equal(t, 1, test.server.count())
		require.Equal(t, "/@alice/inbox", test.server.requests[0].URL.Path)
	})

	t.Run("delivered to the owner when the collection cannot be read", func(t *testing.T) {

		test := newSendTest(t)

		require.Equal(t, queue.ResultStatusSuccess, test.pipeline.Send(test.replyArgs(t, test.alice+"/followers")).Status)
		require.Equal(t, 1, test.server.count())
	})

	t.Run("an excluded owner gets nothing", func(t *testing.T) {

		var asked []string

		test := newSendTest(t, WithFilterOne(func(actorID string, recipientURL string) bool {
			asked = append(asked, actorID+" "+recipientURL)
			return false
		}))

		require.Equal(t, queue.ResultStatusSuccess, test.pipeline.Send(test.replyArgs(t, test.alice+"/followers")).Status)
		require.Zero(t, test.server.count())
		require.Equal(t, []string{testActor + " " + test.alice}, asked)
	})

	t.Run("a collection with no owner fails for good", func(t *testing.T) {

		test := newSendTest(t)
		orphan := test.server.URL + "/collections/orphan"
		test.client.documents[orphan] = map[string]any{"id": orphan, "type": "Collection"}

		require.Equal(t, queue.ResultStatusFailure, test.pipeline.Send(test.replyArgs(t, orphan)).Status)
		require.Zero(t, test.server.count())
	})

	t.Run("a collection owned by a collection fails without looping", func(t *testing.T) {

		test := newSendTest(t)

		// The post's author is itself a collection that claims the first one as its followers
		outer := test.server.URL + "/collections/outer"
		inner := test.server.URL + "/collections/inner"
		parent := test.server.URL + "/notes/2"

		test.client.documents[outer] = map[string]any{"id": outer, "type": "Collection"}
		test.client.documents[inner] = map[string]any{"id": inner, "type": "Collection", "followers": outer}
		test.client.documents[parent] = map[string]any{"id": parent, "type": "Note", "attributedTo": inner}

		body, err := json.Marshal(map[string]any{
			"id": testActor + "/2", "actor": testActor, "cc": []any{outer},
			"object": map[string]any{"inReplyTo": parent},
		})
		require.NoError(t, err)

		result := test.pipeline.Send(mapof.Any{"actor": testActor, "recipient": outer, "body": string(body)})
		require.Equal(t, queue.ResultStatusFailure, result.Status)
		require.Zero(t, test.server.count())
		require.Less(t, len(test.client.loads), 10, "the search for an owner is bounded")
	})
}

// TestConsumer verifies that the consumer runs both tasks and ignores every other
func TestConsumer(t *testing.T) {

	test := newSendTest(t)
	taskQueue, recorder := newRecordingQueue()
	test.pipeline.queue = taskQueue
	consumer := Consumer(test.pipeline)

	fanOut := queue.NewTask(TaskFanOut, mapof.Any{"activity": mapof.Any{"id": testActor + "/1", "actor": testActor, "to": test.alice}})
	require.Equal(t, queue.ResultStatusSuccess, consumer.Run(fanOut).Status)
	require.Len(t, recorder.tasks, 1)

	require.Equal(t, queue.ResultStatusSuccess, consumer.Run(recorder.tasks[0]).Status)
	require.Equal(t, 1, test.server.count())

	require.Equal(t, queue.ResultStatusIgnored, consumer.Run(queue.NewTask("SomethingElse", nil)).Status)

	require.NoError(t, consumer.OnPublish(&fanOut))
	require.NoError(t, consumer.OnSuccess(fanOut))
	require.NoError(t, consumer.OnError(fanOut, nil))
	require.NoError(t, consumer.OnFailure(fanOut, nil))
}
