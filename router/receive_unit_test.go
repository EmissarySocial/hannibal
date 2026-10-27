package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/validator"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubValidator is a Validator that always returns the configured result.
type stubValidator struct {
	result validator.Result
}

func (s stubValidator) Validate(_ *http.Request, _ *streams.Document) validator.Result {
	return s.result
}

const followActivityJSON = `{
	"@context": "https://www.w3.org/ns/activitystreams",
	"id": "https://example.com/activities/1",
	"type": "Follow",
	"actor": "https://example.com/users/alice",
	"object": "https://example.com/users/bob"
}`

/******************************************
 * validateRequest -- the validator-chain logic
 ******************************************/

// TestValidateRequest confirms the short-circuit semantics of the validator
// chain: the first Valid/Invalid result decides, Unknown continues, and an empty
// or all-Unknown chain fails closed.
func TestValidateRequest(t *testing.T) {

	request := newActivityRequest(followActivityJSON)
	document := streams.NewDocument(map[string]any{})

	check := func(name string, validators []Validator, expected bool) {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, expected, validateRequest(request, &document, validators))
		})
	}

	valid := stubValidator{validator.ResultValid}
	invalid := stubValidator{validator.ResultInvalid}
	unknown := stubValidator{validator.ResultUnknown}

	// A single Valid result passes.
	check("single valid", []Validator{valid}, true)

	// A single Invalid result fails.
	check("single invalid", []Validator{invalid}, false)

	// Unknown then Valid -> passes (Unknown continues the chain).
	check("unknown then valid", []Validator{unknown, valid}, true)

	// Unknown then Invalid -> fails.
	check("unknown then invalid", []Validator{unknown, invalid}, false)

	// Invalid wins even if a later validator would say Valid (first decisive
	// result short-circuits).
	check("invalid before valid", []Validator{invalid, valid}, false)

	// An all-Unknown chain fails closed.
	check("all unknown", []Validator{unknown, unknown}, false)

	// An empty validator chain fails closed -- nothing could vouch for it.
	check("empty chain", []Validator{}, false)
}

/******************************************
 * ReceiveRequest
 ******************************************/

// TestReceiveRequest_Valid confirms a well-formed, validated request is parsed
// into the expected activity.
func TestReceiveRequest_Valid(t *testing.T) {

	request := newActivityRequest(followActivityJSON)
	client := streams.NewDefaultClient()

	activity, err := ReceiveRequest(request, client,
		WithValidators(stubValidator{validator.ResultValid}))

	require.NoError(t, err)
	assert.Equal(t, vocab.ActivityTypeFollow, activity.Type())
	assert.Equal(t, "https://example.com/activities/1", activity.ID())
}

// TestReceiveRequest_Rejected confirms a request that fails validation is
// rejected with an error and a nil document.
func TestReceiveRequest_Rejected(t *testing.T) {

	request := newActivityRequest(followActivityJSON)
	client := streams.NewDefaultClient()

	activity, err := ReceiveRequest(request, client,
		WithValidators(stubValidator{validator.ResultInvalid}))

	require.Error(t, err)
	assert.True(t, activity.IsNil())
}

// TestReceiveRequest_BadJSON confirms a malformed or empty body is rejected as a 400 Bad Request
// (a peer's junk POST is a client error), not a codeless error that surfaces as a generic 500.
func TestReceiveRequest_BadJSON(t *testing.T) {

	client := streams.NewDefaultClient()

	check := func(name string, body string) {
		t.Run(name, func(t *testing.T) {
			request := newActivityRequest(body)

			_, err := ReceiveRequest(request, client,
				WithValidators(stubValidator{validator.ResultValid}))

			require.Error(t, err)
			assert.Equal(t, http.StatusBadRequest, derp.ErrorCode(err))
		})
	}

	check("empty body", ``)
	check("not json", `not json at all`)
	check("truncated json", `{ this is not valid json `)
	check("malformed json", `{"@#$%"}`)
}

/******************************************
 * ReceiveAndHandle -- the combined path
 ******************************************/

// TestReceiveAndHandle confirms the end-to-end path: parse + validate + route to
// the matching handler.
func TestReceiveAndHandle(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.ActivityTypeFollow, vocab.Any, handler("follow"))

	request := newActivityRequest(followActivityJSON)
	client := streams.NewDefaultClient()

	context := &capture{}
	err := router.ReceiveAndHandle(context, request, client,
		WithValidators(stubValidator{validator.ResultValid}))

	require.NoError(t, err)
	assert.Equal(t, "follow", context.hit)
}

// TestReceiveAndHandle_HandlerError confirms an error from the matched handler
// is propagated back through ReceiveAndHandle.
func TestReceiveAndHandle_HandlerError(t *testing.T) {

	sentinel := stubError("handler exploded")

	router := New[*capture]()
	router.Add(vocab.ActivityTypeFollow, vocab.Any,
		func(_ *capture, _ streams.Document) error {
			return sentinel
		})

	request := newActivityRequest(followActivityJSON)
	client := streams.NewDefaultClient()

	err := router.ReceiveAndHandle(&capture{}, request, client,
		WithValidators(stubValidator{validator.ResultValid}))

	require.Error(t, err, "a handler error must propagate out of ReceiveAndHandle")
}

// TestReceiveAndHandle_ValidationError confirms a validation failure aborts
// before any handler runs.
func TestReceiveAndHandle_ValidationError(t *testing.T) {

	router := New[*capture]()
	router.Add(vocab.ActivityTypeFollow, vocab.Any, handler("follow"))

	request := newActivityRequest(followActivityJSON)
	client := streams.NewDefaultClient()

	context := &capture{}
	err := router.ReceiveAndHandle(context, request, client,
		WithValidators(stubValidator{validator.ResultInvalid}))

	require.Error(t, err)
	assert.Equal(t, "", context.hit, "no handler should run when validation fails")
}

/******************************************
 * Test Helpers
 ******************************************/

// newActivityRequest builds a POST request whose body is the given JSON activity.
func newActivityRequest(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", strings.NewReader(body))
	request.Header.Set("Content-Type", vocab.ContentTypeActivityPub)
	return request
}

// stubError is a minimal error type for tests.
type stubError string

func (e stubError) Error() string { return string(e) }
