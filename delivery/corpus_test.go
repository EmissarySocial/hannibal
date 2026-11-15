package delivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/benpate/hannibal/corpus"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/remote"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/benpate/turbine/queue"
	"github.com/stretchr/testify/require"
)

/******************************************
 * Corpus Tests
 *
 * These run every document in the corpus package through the
 * fan-out and the send. A fediverse example is checked against
 * the recipients recorded beside it. A W3C example records none,
 * so it is checked against a second, simpler reading of its
 * addressing that is written here.
 ******************************************/

// corpusBob is an actor that the fediverse examples name directly, and who also follows the author
const corpusBob = "https://mastodon.example.org/users/bob"

// corpusWorld is the sending actor of every fediverse example, and its three followers
var corpusWorld = corpus.World{
	Actor:     testActor,
	Followers: testFollowers,
	Members:   []string{"https://follower.example.org/users/alice", corpusBob, "mailto:dana@example.org"},
}

// corpusMap is a named map type, which the walk can only read by reflection
type corpusMap map[string]any

// corpusList is a named slice type, which the walk can only read by reflection
type corpusList []any

// corpusShapes are the Go types that one JSON document can arrive as. Each returns the
// document rebuilt with its own map and slice types below the top level.
var corpusShapes = map[string]func(any) any{
	"json":  func(value any) any { return value },
	"mapof": func(value any) any { return reshape(value, toMapOfAny, toSliceOfAny) },
	"named": func(value any) any { return reshape(value, toCorpusMap, toCorpusList) },
}

// TestCorpus_Fediverse_FanOut verifies that every fediverse example reaches the recipients
// recorded for it, in each shape, and is sent without its blind recipients
func TestCorpus_Fediverse_FanOut(t *testing.T) {

	examples, err := corpus.Fediverse(corpusWorld)
	require.NoError(t, err)

	for _, example := range examples {
		for shapeName, shape := range corpusShapes {
			t.Run(example.Name+"/"+shapeName, func(t *testing.T) {

				outcome := fanOutExample(t, example, shape, testSigners)

				addresses, emails := splitEmails(example.Recipients)
				require.Equal(t, addresses, outcome.recorder.recipients(), "send tasks")
				require.Equal(t, emails, outcome.emails, "email recipients")

				// RULE: Nothing from the blind.example.org host is ever in what is sent
				for _, task := range outcome.recorder.tasks {
					require.NotContains(t, task.Arguments.GetString("body"), "blind.example.org")
				}
			})
		}
	}
}

// TestCorpus_Fediverse_Send verifies that every send task queued for a fediverse example
// delivers its body unchanged, in a signed request, to the inbox of the actor it names
func TestCorpus_Fediverse_Send(t *testing.T) {

	examples, err := corpus.Fediverse(corpusWorld)
	require.NoError(t, err)

	for _, example := range examples {
		t.Run(example.Name, func(t *testing.T) {

			outcome := fanOutExample(t, example, corpusShapes["json"], testSigners)

			// Deliver every queued task through a pipeline that records each request
			interceptor := &requestRecorder{}
			pipeline := New(nil, corpusClient{}, testSigners, WithRemoteOptions(interceptor.options))

			expectedInboxes := make([]string, 0, len(outcome.recorder.tasks))

			for _, task := range outcome.recorder.tasks {
				result := pipeline.Send(task.Arguments)
				require.Equal(t, queue.ResultStatusSuccess, result.Status, "%v", result.Error)
				expectedInboxes = append(expectedInboxes, corpusInbox(task.Arguments.GetString("recipient")))
			}

			require.Equal(t, expectedInboxes, interceptor.urls())

			for index, request := range interceptor.requests {

				task := outcome.recorder.tasks[index]
				body := interceptor.bodies[index]

				// The inbox receives the bytes that the fan-out queued
				require.Equal(t, task.Arguments.GetString("body"), body)
				require.NoError(t, noBlindKeys(decodeJSON(t, []byte(body))))

				// The signature covers a digest of those same bytes
				hash := sha256.Sum256([]byte(body))
				require.Equal(t, "SHA-256="+base64.StdEncoding.EncodeToString(hash[:]), request.Header.Get("Digest"))
				require.Contains(t, request.Header.Get("Signature"), `keyId="`+testActor+`#main-key"`)
				require.Contains(t, request.Header.Get("Signature"), `headers="(request-target) host date digest"`)
			}
		})
	}
}

// TestCorpus_W3C_FanOut verifies that every document the W3C publishes passes through the
// fan-out: it reaches the recipients it names, and is sent whole apart from its blind recipients
func TestCorpus_W3C_FanOut(t *testing.T) {

	published, err := corpus.W3C()
	require.NoError(t, err)

	rejected, err := corpus.Rejected()
	require.NoError(t, err)

	// These documents name many actors, and several name none, so every actor may send
	anySigner := func(string) (Signer, error) { return testSigner{}, nil }

	for _, example := range append(published, rejected...) {
		for shapeName, shape := range corpusShapes {
			t.Run(example.Name+"/"+shapeName, func(t *testing.T) {

				outcome := fanOutExample(t, example, shape, anySigner)

				addresses, emails := splitEmails(namedRecipients(decodeJSON(t, example.JSON)))
				require.Equal(t, addresses, outcome.recorder.recipients(), "send tasks")
				require.Equal(t, emails, outcome.emails, "email recipients")
			})
		}
	}
}

// TestCorpus_W3C_HasBlindRecipients verifies that the W3C documents include some with blind
// recipients, so that the test above cannot pass by never meeting one
func TestCorpus_W3C_HasBlindRecipients(t *testing.T) {

	published, err := corpus.W3C()
	require.NoError(t, err)

	count := 0

	for _, example := range published {
		if noBlindKeys(decodeJSON(t, example.JSON)) != nil {
			count++
		}
	}

	require.GreaterOrEqual(t, count, 2)
}

/******************************************
 * Corpus Helpers
 ******************************************/

// fanOutOutcome is what one fan-out queued and emailed
type fanOutOutcome struct {
	recorder *taskRecorder
	emails   []string
}

// fanOutExample runs one example through FanOut in the given shape, and checks everything that
// holds for any document: one body for all, no blind recipients in it, and an untouched original
func fanOutExample(t *testing.T, example corpus.Example, shape func(any) any, signers func(string) (Signer, error)) fanOutOutcome {

	t.Helper()

	taskQueue, recorder := newRecordingQueue()
	emails := make([]string, 0)
	emailed := make([]map[string]any, 0)

	pipeline := New(taskQueue, &documentClient{}, signers,
		WithCollections(&testCollections{members: map[string][]string{corpusWorld.Followers: corpusWorld.Members}}),
		WithEmail(func(_ string, address string, activity map[string]any) error {
			emails = append(emails, address)
			emailed = append(emailed, activity)
			return nil
		}),
	)

	document, err := example.Document()
	require.NoError(t, err)

	activity, isMap := shape(document).(map[string]any)
	require.True(t, isMap)

	result := pipeline.FanOut(mapof.Any{"activity": activity})
	require.Equal(t, queue.ResultStatusSuccess, result.Status, "%v", result.Error)

	// What is sent is the document without its blind recipients, and nothing else is changed
	expected := withoutBlindKeys(decodeJSON(t, example.JSON))
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)

	activityID, _ := document["id"].(string)

	for _, task := range recorder.tasks {

		body := task.Arguments.GetString("body")
		require.JSONEq(t, string(expectedJSON), body)
		require.Equal(t, recorder.tasks[0].Arguments.GetString("body"), body, "every recipient is sent the same bytes")

		hash := sha256.Sum256([]byte(body))
		require.Equal(t, "SHA-256="+base64.StdEncoding.EncodeToString(hash[:]), task.Arguments.GetString("digest"))

		// A document with an id of its own is queued under it
		if activityID != "" {
			require.Equal(t, activityID+" "+task.Arguments.GetString("recipient"), task.Signature)
		}
	}

	for _, emailedActivity := range emailed {
		actualJSON, err := json.Marshal(emailedActivity)
		require.NoError(t, err)
		require.JSONEq(t, string(expectedJSON), string(actualJSON))
	}

	// RULE: The caller's document keeps its blind recipients, because the stored copy is still read
	afterJSON, err := json.Marshal(activity)
	require.NoError(t, err)
	require.JSONEq(t, string(example.JSON), string(afterJSON), "the fan-out changed the caller's document")

	return fanOutOutcome{recorder: recorder, emails: emails}
}

// decodeJSON returns a document decoded as plain JSON
func decodeJSON(t *testing.T, document []byte) map[string]any {

	t.Helper()

	result := make(map[string]any)
	require.NoError(t, json.Unmarshal(document, &result))
	return result
}

// withoutBlindKeys deletes `bto` and `bcc` from every object in a decoded JSON value, and
// returns it. It is the simplest strip there is, written apart from the one under test.
func withoutBlindKeys(value any) any {

	switch typed := value.(type) {

	case map[string]any:
		delete(typed, "bto")
		delete(typed, "bcc")
		for _, item := range typed {
			withoutBlindKeys(item)
		}

	case []any:
		for _, item := range typed {
			withoutBlindKeys(item)
		}
	}

	return value
}

// noBlindKeys returns an error if any object in a decoded JSON value has a `bto` or `bcc`
func noBlindKeys(value any) error {

	switch typed := value.(type) {

	case map[string]any:
		for key, item := range typed {
			if (key == "bto") || (key == "bcc") {
				return io.ErrUnexpectedEOF
			}
			if err := noBlindKeys(item); err != nil {
				return err
			}
		}

	case []any:
		for _, item := range typed {
			if err := noBlindKeys(item); err != nil {
				return err
			}
		}
	}

	return nil
}

// namedRecipients reads the addressing of a decoded JSON document the plain way: every id in
// `to`, `cc`, `bto` and `bcc`, once each, without Public and without anything that has no host
func namedRecipients(document map[string]any) []string {

	result := make([]string, 0)

	var read func(value any)

	read = func(value any) {

		switch typed := value.(type) {

		case string:
			if isDeliverable(typed) && !slices.Contains(result, typed) {
				result = append(result, typed)
			}

		case map[string]any:
			if id, isString := typed["id"].(string); isString {
				read(id)
			}

		case []any:
			for _, item := range typed {
				read(item)
			}
		}
	}

	for _, field := range []string{"to", "cc", "bto", "bcc"} {
		read(document[field])
	}

	return result
}

// isDeliverable returns TRUE if an address is an email address or a URL with a host, and is not Public
func isDeliverable(address string) bool {

	switch address {
	case "https://www.w3.org/ns/activitystreams#Public", "as:Public", "Public":
		return false
	}

	if strings.HasPrefix(address, "mailto:") {
		return true
	}

	parsed, err := url.Parse(address)

	if (err != nil) || (parsed == nil) {
		return false
	}

	return parsed.Host != ""
}

// splitEmails separates a list of recipients into the URLs that get a send task and the
// mailto: URLs that are handed to the application
func splitEmails(recipients []string) ([]string, []string) {

	addresses := make([]string, 0, len(recipients))
	emails := make([]string, 0)

	for _, recipient := range recipients {
		if strings.HasPrefix(recipient, "mailto:") {
			emails = append(emails, recipient)
			continue
		}
		addresses = append(addresses, recipient)
	}

	return addresses, emails
}

// reshape rebuilds a decoded JSON value with other map and slice types below the top level,
// which stays a plain map because that is what a queue task holds
func reshape(value any, toMap func(map[string]any) any, toSlice func([]any) any) any {

	top, isMap := value.(map[string]any)

	if !isMap {
		return value
	}

	result := make(map[string]any, len(top))

	for key, item := range top {
		result[key] = reshapeInner(item, toMap, toSlice)
	}

	return result
}

// reshapeInner rebuilds one nested value, and everything inside it
func reshapeInner(value any, toMap func(map[string]any) any, toSlice func([]any) any) any {

	switch typed := value.(type) {

	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = reshapeInner(item, toMap, toSlice)
		}
		return toMap(result)

	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = reshapeInner(item, toMap, toSlice)
		}
		return toSlice(result)
	}

	return value
}

// toMapOfAny returns a map as rosetta's mapof.Any
func toMapOfAny(value map[string]any) any { return mapof.Any(value) }

// toSliceOfAny returns a slice as rosetta's sliceof.Any
func toSliceOfAny(value []any) any { return sliceof.Any(value) }

// toCorpusMap returns a map as a named type that only reflection can read
func toCorpusMap(value map[string]any) any { return corpusMap(value) }

// toCorpusList returns a slice as a named type that only reflection can read
func toCorpusList(value []any) any { return corpusList(value) }

// corpusClient is a streams.Client in which every URL is an actor with an inbox on its own
// host, except a URL that ends in /followers, which is that actor's collection
type corpusClient struct{}

// SetRootClient implements streams.Client
func (corpusClient) SetRootClient(streams.Client) {}

// Load returns the actor or the collection that a URL names
func (client corpusClient) Load(documentURL string, _ ...any) (streams.Document, error) {

	if strings.HasSuffix(documentURL, "/followers") {
		return streams.NewDocument(map[string]any{"id": documentURL, "type": "OrderedCollection"}, streams.WithClient(client)), nil
	}

	return streams.NewDocument(map[string]any{
		"id":        documentURL,
		"type":      "Person",
		"inbox":     documentURL + "/inbox",
		"followers": documentURL + "/followers",
	}, streams.WithClient(client)), nil
}

// Save implements streams.Client
func (corpusClient) Save(streams.Document) error { return nil }

// Delete implements streams.Client
func (corpusClient) Delete(string) error { return nil }

// corpusInbox returns the inbox that a recipient's copy should reach: the actor's own, or the
// inbox of the actor who owns a followers collection
func corpusInbox(recipientURL string) string {
	return strings.TrimSuffix(recipientURL, "/followers") + "/inbox"
}

// requestRecorder answers every request with 202 before it leaves the process, and keeps a
// copy of each one as it was signed
type requestRecorder struct {
	mutex    sync.Mutex
	requests []*http.Request
	bodies   []string
}

// options returns the remote option that records and answers a request
func (recorder *requestRecorder) options() []remote.Option {

	return []remote.Option{{
		ModifyRequest: func(_ *remote.Transaction, request *http.Request) (*http.Response, error) {

			body, err := io.ReadAll(request.Body)

			if err != nil {
				return nil, err
			}

			recorder.mutex.Lock()
			defer recorder.mutex.Unlock()

			recorder.requests = append(recorder.requests, request)
			recorder.bodies = append(recorder.bodies, string(body))

			return &http.Response{
				StatusCode: http.StatusAccepted,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewReader(nil)),
				Request:    request,
			}, nil
		},
	}}
}

// urls returns the address of every recorded request, in order
func (recorder *requestRecorder) urls() []string {

	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()

	result := make([]string, 0, len(recorder.requests))

	for _, request := range recorder.requests {
		result = append(result, request.URL.String())
	}

	return result
}
