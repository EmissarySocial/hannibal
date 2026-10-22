package streams

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/require"
)

const (
	policyTestAuthor    = "https://social.example/users/author"
	policyTestStranger  = "https://other.example/users/stranger"
	policyTestMentioned = "https://other.example/users/mentioned"
	policyTestFollowers = "https://social.example/users/author/followers"
)

// policyTestDecode decodes a JSON interactionPolicy the way it arrives off the wire
func policyTestDecode(t *testing.T, body string) InteractionPolicy {

	t.Helper()

	result := InteractionPolicy{}
	require.NoError(t, json.Unmarshal([]byte(body), &result))
	return result
}

// TestNewInteractionPolicy_Coerces pins that every JSON-LD shape becomes a map
func TestNewInteractionPolicy_Coerces(t *testing.T) {

	canReply := map[string]any{vocab.PropertyCanReply: map[string]any{vocab.PropertyAutomaticApproval: policyTestAuthor}}

	require.Equal(t, mapof.NewAny(), NewInteractionPolicy(nil).Map())
	require.Equal(t, mapof.NewAny(), NewInteractionPolicy(42).Map())
	require.Equal(t, mapof.NewAny(), NewInteractionPolicy([]any{}).Map())
	require.Equal(t, mapof.Any(canReply), NewInteractionPolicy(canReply).Map())
	require.Equal(t, mapof.Any(canReply), NewInteractionPolicy(mapof.Any(canReply)).Map())
	require.Equal(t, mapof.Any(canReply), NewInteractionPolicy([]any{canReply}).Map(), "a list uses its first item")
	require.Equal(t, mapof.Any(canReply), NewInteractionPolicy(NewDocument(canReply)).Map())
	require.Equal(t, mapof.Any{vocab.PropertyID: "https://social.example/policy"}, NewInteractionPolicy("https://social.example/policy").Map(), "an IRI keeps its id")
	require.Equal(t, "https://social.example/policy", NewInteractionPolicy("https://social.example/policy").ID())
}

// TestInteractionPolicy_JSON pins that UnmarshalJSON accepts any JSON value, and that MarshalJSON round-trips
func TestInteractionPolicy_JSON(t *testing.T) {

	policy := policyTestDecode(t, `{"canReply": {"automaticApproval": ["`+policyTestAuthor+`"]}}`)
	require.Equal(t, sliceof.String{policyTestAuthor}, policy.AutomaticApproval(vocab.PropertyCanReply))

	require.True(t, policyTestDecode(t, `null`).IsNil())
	require.True(t, policyTestDecode(t, `7`).IsNil())
	require.True(t, policyTestDecode(t, `"https://social.example/policy"`).IsNil())
	require.True(t, policyTestDecode(t, `[{"canLike": {"manualApproval": []}}]`).Defines(vocab.PropertyCanLike))

	require.Error(t, json.Unmarshal([]byte(`{`), &policy))

	encoded, err := json.Marshal(policy)
	require.NoError(t, err)
	require.JSONEq(t, `{"canReply": {"automaticApproval": ["`+policyTestAuthor+`"]}}`, string(encoded))

	// A struct field decodes through UnmarshalJSON too
	wrapper := struct {
		InteractionPolicy InteractionPolicy `json:"interactionPolicy"`
	}{}
	require.NoError(t, json.Unmarshal([]byte(`{"interactionPolicy": {"canQuote": {"automaticApproval": "`+policyTestAuthor+`"}}}`), &wrapper))
	require.True(t, wrapper.InteractionPolicy.Defines(vocab.PropertyCanQuote))
}

// TestInteractionPolicy_Lists pins how each approval list is read, including GoToSocial's legacy keys
func TestInteractionPolicy_Lists(t *testing.T) {

	policy := policyTestDecode(t, `{
		"canReply": {"automaticApproval": "`+policyTestAuthor+`", "manualApproval": [{"id": "`+policyTestFollowers+`"}, "", "`+policyTestStranger+`"]},
		"canLike": {"always": ["`+policyTestAuthor+`"], "approvalRequired": ["`+policyTestStranger+`"]},
		"canAnnounce": {"automaticApproval": [], "always": ["`+policyTestStranger+`"]},
		"canQuote": {}
	}`)

	require.Equal(t, sliceof.String{policyTestAuthor}, policy.AutomaticApproval(vocab.PropertyCanReply), "a single string")
	require.Equal(t, sliceof.String{policyTestFollowers, policyTestStranger}, policy.ManualApproval(vocab.PropertyCanReply), "objects and strings, without empties")
	require.Equal(t, sliceof.String{policyTestAuthor}, policy.AutomaticApproval(vocab.PropertyCanLike), "legacy always")
	require.Equal(t, sliceof.String{policyTestStranger}, policy.ManualApproval(vocab.PropertyCanLike), "legacy approvalRequired")
	require.Empty(t, policy.AutomaticApproval(vocab.PropertyCanAnnounce), "the current key wins over the legacy key, even when empty")

	nulls := policyTestDecode(t, `{"canReply": {"automaticApproval": null, "always": ["`+policyTestAuthor+`"], "manualApproval": null}}`)
	require.Equal(t, sliceof.String{policyTestAuthor}, nulls.AutomaticApproval(vocab.PropertyCanReply), "a null current key falls back to the legacy key")
	require.Empty(t, nulls.ManualApproval(vocab.PropertyCanReply))
	require.False(t, policyTestDecode(t, `{"canReply": {"automaticApproval": null}}`).Defines(vocab.PropertyCanReply), "a null list is not defined")

	ids := policyTestDecode(t, `{"canLike": {"automaticApproval": [{"id": "`+policyTestAuthor+`", "@id": "https://ignored.example"}, {"@id": "`+policyTestStranger+`"}, {"type": "Person"}]}}`)
	require.Equal(t, sliceof.String{policyTestAuthor, policyTestStranger}, ids.AutomaticApproval(vocab.PropertyCanLike), "id wins over @id, and an object with neither is skipped")

	require.True(t, policy.Defines(vocab.PropertyCanReply))
	require.True(t, policy.Defines(vocab.PropertyCanLike))
	require.True(t, policy.Defines(vocab.PropertyCanAnnounce), "an empty list is still defined")
	require.False(t, policy.Defines(vocab.PropertyCanQuote), "an empty sub-policy is not")
	require.True(t, policy.NotNil())
}

// TestInteractionPolicy_CanInteract pins GoToSocial's and FEP-044f's rules for each kind of actor
func TestInteractionPolicy_CanInteract(t *testing.T) {

	request := func(actorID string, isFollower bool) InteractionRequest {
		return InteractionRequest{
			ActorID:   actorID,
			AuthorID:  policyTestAuthor,
			Mentioned: []string{policyTestMentioned},
			IsMember: func(collectionID string) bool {
				return isFollower && (collectionID == policyTestFollowers)
			},
		}
	}

	stranger := request(policyTestStranger, false)
	follower := request(policyTestStranger, true)
	author := request(policyTestAuthor, false)
	mentioned := request(policyTestMentioned, false)
	anonymous := request("", false)

	authorOnly := `{"automaticApproval": ["` + policyTestAuthor + `"]}`

	testCases := []struct {
		name       string
		policy     string
		permission string
		request    InteractionRequest
		expected   InteractionResult
	}{
		// Defaults
		{"no policy, reply", `{}`, vocab.PropertyCanReply, stranger, InteractionAllowed},
		{"no policy, like", `{}`, vocab.PropertyCanLike, stranger, InteractionAllowed},
		{"no policy, announce", `{}`, vocab.PropertyCanAnnounce, stranger, InteractionAllowed},
		{"no policy, quote", `{}`, vocab.PropertyCanQuote, stranger, InteractionDenied},
		{"no policy, self-quote", `{}`, vocab.PropertyCanQuote, author, InteractionAllowed},
		{"an empty sub-policy is the default", `{"canReply": {}}`, vocab.PropertyCanReply, stranger, InteractionAllowed},
		{"an anonymous actor", `{}`, vocab.PropertyCanReply, anonymous, InteractionDenied},

		// The author and mentioned actors
		{"closed, a stranger", `{"canReply": ` + authorOnly + `}`, vocab.PropertyCanReply, stranger, InteractionDenied},
		{"closed, the author", `{"canReply": ` + authorOnly + `}`, vocab.PropertyCanReply, author, InteractionAllowed},
		{"closed, a mentioned actor replies", `{"canReply": ` + authorOnly + `}`, vocab.PropertyCanReply, mentioned, InteractionAllowed},
		{"closed, a mentioned actor likes", `{"canLike": ` + authorOnly + `}`, vocab.PropertyCanLike, mentioned, InteractionDenied},
		{"closed, a mentioned actor quotes", `{"canQuote": ` + authorOnly + `}`, vocab.PropertyCanQuote, mentioned, InteractionDenied},
		{"an empty list, a stranger", `{"canReply": {"automaticApproval": []}}`, vocab.PropertyCanReply, stranger, InteractionDenied},

		// Audiences
		{"Public", `{"canQuote": {"automaticApproval": ["https://www.w3.org/ns/activitystreams#Public"]}}`, vocab.PropertyCanQuote, stranger, InteractionAllowed},
		{"compact Public", `{"canReply": {"automaticApproval": ["as:Public"]}}`, vocab.PropertyCanReply, stranger, InteractionAllowed},
		{"Public with approval", `{"canReply": {"automaticApproval": ["` + policyTestAuthor + `"], "manualApproval": ["https://www.w3.org/ns/activitystreams#Public"]}}`, vocab.PropertyCanReply, stranger, InteractionNeedsApproval},
		{"followers, a follower", `{"canReply": {"automaticApproval": ["` + policyTestFollowers + `"]}}`, vocab.PropertyCanReply, follower, InteractionAllowed},
		{"followers, a stranger", `{"canReply": {"automaticApproval": ["` + policyTestFollowers + `"]}}`, vocab.PropertyCanReply, stranger, InteractionDenied},
		{"a named actor", `{"canAnnounce": {"automaticApproval": ["` + policyTestStranger + `"]}}`, vocab.PropertyCanAnnounce, stranger, InteractionAllowed},
		{"a named actor with approval", `{"canAnnounce": {"manualApproval": ["` + policyTestStranger + `"]}}`, vocab.PropertyCanAnnounce, stranger, InteractionNeedsApproval},
		{"legacy keys", `{"canReply": {"always": ["` + policyTestAuthor + `"], "approvalRequired": ["as:Public"]}}`, vocab.PropertyCanReply, stranger, InteractionNeedsApproval},
		{"an id object", `{"canReply": {"automaticApproval": [{"id": "` + policyTestStranger + `"}]}}`, vocab.PropertyCanReply, stranger, InteractionAllowed},
		{"an @id object", `{"canReply": {"automaticApproval": [{"@id": "` + policyTestStranger + `"}]}}`, vocab.PropertyCanReply, stranger, InteractionAllowed},

		// JSON null means unset
		{"a null list is the default", `{"canReply": {"automaticApproval": null}}`, vocab.PropertyCanReply, stranger, InteractionAllowed},
		{"a null list falls back to the legacy key", `{"canReply": {"automaticApproval": null, "always": ["` + policyTestAuthor + `"]}}`, vocab.PropertyCanReply, stranger, InteractionDenied},
		{"a null sub-policy is the default", `{"canReply": null}`, vocab.PropertyCanReply, stranger, InteractionAllowed},
		{"a null quote sub-policy is the default", `{"canQuote": null}`, vocab.PropertyCanQuote, stranger, InteractionDenied},

		// Precedence
		{"an actor is more specific than Public", `{"canReply": {"automaticApproval": ["as:Public"], "manualApproval": ["` + policyTestStranger + `"]}}`, vocab.PropertyCanReply, stranger, InteractionNeedsApproval},
		{"a collection is more specific than Public", `{"canReply": {"automaticApproval": ["as:Public"], "manualApproval": ["` + policyTestFollowers + `"]}}`, vocab.PropertyCanReply, follower, InteractionNeedsApproval},
		{"an actor is more specific than a collection", `{"canReply": {"automaticApproval": ["` + policyTestStranger + `"], "manualApproval": ["` + policyTestFollowers + `"]}}`, vocab.PropertyCanReply, follower, InteractionAllowed},
		{"automatic wins at the same level", `{"canReply": {"automaticApproval": ["` + policyTestStranger + `"], "manualApproval": ["` + policyTestStranger + `"]}}`, vocab.PropertyCanReply, stranger, InteractionAllowed},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			policy := policyTestDecode(t, testCase.policy)
			require.Equal(t, testCase.expected, policy.CanInteract(testCase.permission, testCase.request))
		})
	}
}

// TestInteractionPolicy_CanInteract_LooksUpOnlyWhatItNeeds pins that IsMember is called only for
// collection IRIs, and only when nothing more specific has decided
func TestInteractionPolicy_CanInteract_LooksUpOnlyWhatItNeeds(t *testing.T) {

	calls := sliceof.NewString()
	request := InteractionRequest{
		ActorID:  policyTestStranger,
		AuthorID: policyTestAuthor,
		IsMember: func(collectionID string) bool { calls = append(calls, collectionID); return false },
	}

	open := policyTestDecode(t, `{"canReply": {"automaticApproval": ["as:Public", "`+policyTestAuthor+`"]}}`)
	require.Equal(t, InteractionAllowed, open.CanInteract(vocab.PropertyCanReply, request))
	require.Empty(t, calls, "Public and the author are not collections")

	named := policyTestDecode(t, `{"canReply": {"automaticApproval": ["`+policyTestStranger+`", "`+policyTestFollowers+`"]}}`)
	require.Equal(t, InteractionAllowed, named.CanInteract(vocab.PropertyCanReply, request))
	require.Empty(t, calls, "the actor's own IRI decides first")

	followers := policyTestDecode(t, `{"canReply": {"automaticApproval": ["`+policyTestFollowers+`"]}}`)
	require.Equal(t, InteractionDenied, followers.CanInteract(vocab.PropertyCanReply, request))
	require.Equal(t, sliceof.String{policyTestFollowers}, calls)

	request.IsMember = nil
	require.Equal(t, InteractionDenied, followers.CanInteract(vocab.PropertyCanReply, request), "no IsMember means no memberships")
}

// TestDocument_InteractionPolicy pins that the accessor reads an inline policy, and never loads one published as an IRI
func TestDocument_InteractionPolicy(t *testing.T) {

	const policyID = "https://example.com/note/1/policy"

	inline := NewDocument(map[string]any{
		vocab.PropertyID:                "https://example.com/note/1",
		vocab.PropertyInteractionPolicy: map[string]any{vocab.PropertyCanReply: map[string]any{vocab.PropertyAutomaticApproval: policyTestAuthor}},
	})
	require.Equal(t, sliceof.String{policyTestAuthor}, inline.InteractionPolicy().AutomaticApproval(vocab.PropertyCanReply))

	client := newRecordingClient()
	client.documents[policyID] = map[string]any{
		vocab.PropertyID:       policyID,
		vocab.PropertyCanReply: map[string]any{vocab.PropertyAutomaticApproval: []any{policyTestAuthor}},
	}

	linked := NewDocument(map[string]any{
		vocab.PropertyID:                "https://example.com/note/1",
		vocab.PropertyInteractionPolicy: policyID,
	}, WithClient(client))

	policy := linked.InteractionPolicy()
	require.True(t, policy.IsNil(), "an IRI is not a policy")
	require.Equal(t, policyID, policy.ID())
	require.Equal(t, InteractionAllowed, policy.CanInteract(vocab.PropertyCanReply, InteractionRequest{ActorID: policyTestStranger}), "and allows the defaults")

	client.mutex.Lock()
	_, loaded := client.options[policyID]
	client.mutex.Unlock()
	require.False(t, loaded, "the IRI is never loaded")

	require.True(t, NewDocument(map[string]any{vocab.PropertyID: "https://example.com/note/1"}).InteractionPolicy().IsNil())
}

// TestDocument_Quote pins FEP-044f's quote property and the compatibility properties it falls back to
func TestDocument_Quote(t *testing.T) {

	const quoted = "https://social.example/notes/quoted"

	for _, property := range []string{vocab.PropertyQuote, vocab.PropertyQuoteURL, vocab.PropertyQuoteURI, vocab.PropertyMisskeyQuote} {
		document := NewDocument(map[string]any{property: quoted})
		require.Equal(t, quoted, document.Quote().ID(), property)
	}

	preferred := NewDocument(map[string]any{vocab.PropertyQuote: quoted, vocab.PropertyQuoteURL: "https://social.example/notes/other"})
	require.Equal(t, quoted, preferred.Quote().ID(), "quote wins over the compatibility properties")

	require.True(t, NewDocument(map[string]any{}).Quote().IsNil())

	authorization := NewDocument(map[string]any{
		vocab.PropertyQuoteAuthorization: map[string]any{
			vocab.PropertyType:              vocab.ObjectTypeQuoteAuthorization,
			vocab.PropertyInteractingObject: "https://other.example/notes/quoting",
			vocab.PropertyInteractionTarget: quoted,
		},
	}).QuoteAuthorization()

	require.Equal(t, vocab.ObjectTypeQuoteAuthorization, authorization.Type())
	require.Equal(t, "https://other.example/notes/quoting", authorization.InteractingObject().ID())
	require.Equal(t, quoted, authorization.InteractionTarget().ID())
}

// TestInteractionPolicy_ZeroValue pins that an unset InteractionPolicy is usable, and allows GoToSocial's defaults
func TestInteractionPolicy_ZeroValue(t *testing.T) {

	policy := InteractionPolicy{}

	require.NotNil(t, policy.Map())
	require.Empty(t, policy.Map())
	require.Equal(t, "", policy.ID())
	require.True(t, policy.IsNil())
	require.False(t, policy.NotNil())
	require.Empty(t, policy.AutomaticApproval(vocab.PropertyCanReply))
	require.Equal(t, InteractionAllowed, policy.CanInteract(vocab.PropertyCanReply, InteractionRequest{ActorID: policyTestStranger}))

	encoded, err := json.Marshal(policy)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(encoded))
}

// TestInteractionPolicy_MarshalJSON_Error pins that a value JSON cannot encode returns an error, not a panic
func TestInteractionPolicy_MarshalJSON_Error(t *testing.T) {

	policy := NewInteractionPolicy(map[string]any{vocab.PropertyCanReply: make(chan int)})

	_, err := policy.MarshalJSON()
	require.Error(t, err)
}

// TestInteractionMap_Shapes pins every Go value interactionMap accepts
func TestInteractionMap_Shapes(t *testing.T) {

	canLike := mapof.Any{vocab.PropertyCanLike: map[string]any{vocab.PropertyAutomaticApproval: policyTestAuthor}}

	require.Equal(t, canLike, interactionMap(NewInteractionPolicy(canLike)), "an InteractionPolicy")
	require.Equal(t, canLike, interactionMap(property.NewValue(map[string]any(canLike))), "a property.Value")
	require.Equal(t, canLike, interactionMap(sliceof.Any{map[string]any(canLike)}), "a sliceof.Any")
	require.Equal(t, mapof.NewAny(), interactionMap(""), "an empty string")
	require.Equal(t, mapof.NewAny(), interactionMap(true), "a boolean")
}

// TestInteractionIDs_Shapes pins every Go value interactionIDs accepts
func TestInteractionIDs_Shapes(t *testing.T) {

	both := sliceof.String{policyTestAuthor, policyTestStranger}

	require.Equal(t, both, interactionIDs(NewDocument([]any{policyTestAuthor, policyTestStranger})), "a Document")
	require.Equal(t, both, interactionIDs(property.NewValue([]any{policyTestAuthor, policyTestStranger})), "a property.Value")
	require.Equal(t, sliceof.String{policyTestAuthor}, interactionIDs(mapof.Any{vocab.PropertyID: policyTestAuthor}), "a mapof.Any")
	require.Equal(t, both, interactionIDs([]string{policyTestAuthor, "", policyTestStranger}), "a []string, without empties")
	require.Equal(t, both, interactionIDs(sliceof.String{policyTestAuthor, policyTestStranger}), "a sliceof.String")
	require.Equal(t, both, interactionIDs(sliceof.Any{policyTestAuthor, map[string]any{vocab.PropertyID_Alternate: policyTestStranger}}), "a sliceof.Any")
	require.Equal(t, sliceof.String{policyTestAuthor}, interactionIDs([]any{[]any{policyTestAuthor}}), "a nested list")
	require.Empty(t, interactionIDs(42), "a number")
	require.Empty(t, interactionIDs(nil), "nil")
	require.Empty(t, interactionIDs(map[string]any{vocab.PropertyID: 42}), "an id that is not an IRI")
}

// ExampleInteractionPolicy_CanInteract shows how a reply is decided against a post's policy
func ExampleInteractionPolicy_CanInteract() {

	post := NewDocument(map[string]any{
		vocab.PropertyID:           "https://social.example/notes/1",
		vocab.PropertyAttributedTo: "https://social.example/users/author",
		vocab.PropertyInteractionPolicy: map[string]any{
			vocab.PropertyCanReply: map[string]any{
				vocab.PropertyAutomaticApproval: []any{"https://social.example/users/author/followers"},
				vocab.PropertyManualApproval:    []any{vocab.NamespaceActivityStreamsPublic},
			},
		},
	})

	request := func(actorID string, isFollower bool) InteractionRequest {
		return InteractionRequest{
			ActorID:  actorID,
			AuthorID: post.AttributedTo().ID(),
			IsMember: func(collectionID string) bool {
				return isFollower && (collectionID == "https://social.example/users/author/followers")
			},
		}
	}

	policy := post.InteractionPolicy()
	fmt.Println(policy.CanInteract(vocab.PropertyCanReply, request("https://other.example/users/follower", true)) == InteractionAllowed)
	fmt.Println(policy.CanInteract(vocab.PropertyCanReply, request("https://other.example/users/stranger", false)) == InteractionNeedsApproval)
	fmt.Println(policy.CanInteract(vocab.PropertyCanQuote, request("https://other.example/users/follower", true)) == InteractionDenied)

	// Output:
	// true
	// true
	// true
}

// policyTestList and policyTestMap are named types, the way MongoDB's driver returns arrays (primitive.A)
// and documents (primitive.M); a type switch on []any or map[string]any matches neither
type policyTestList []any

type policyTestMap map[string]any

// TestInteractionPolicy_NamedTypes pins that a policy read back from a database, with named slice and
// map types at any level, decides the same way as one decoded from JSON
func TestInteractionPolicy_NamedTypes(t *testing.T) {

	policy := NewInteractionPolicy(policyTestMap{
		vocab.PropertyCanReply: policyTestMap{
			vocab.PropertyAutomaticApproval: policyTestList{vocab.NamespaceActivityStreamsPublic},
			vocab.PropertyManualApproval:    policyTestList{policyTestMap{vocab.PropertyID: policyTestStranger}},
		},
		vocab.PropertyCanLike: policyTestList{mapof.Any{vocab.PropertyAutomaticApproval: []string{policyTestAuthor}}},
	})

	require.True(t, policy.Defines(vocab.PropertyCanReply))
	require.Equal(t, sliceof.String{vocab.NamespaceActivityStreamsPublic}, policy.AutomaticApproval(vocab.PropertyCanReply))
	require.Equal(t, sliceof.String{policyTestStranger}, policy.ManualApproval(vocab.PropertyCanReply))
	require.Equal(t, sliceof.String{policyTestAuthor}, policy.AutomaticApproval(vocab.PropertyCanLike), "a named list of maps uses its first item")

	other := InteractionRequest{ActorID: "https://other.example/users/someone"}
	require.Equal(t, InteractionAllowed, policy.CanInteract(vocab.PropertyCanReply, other))
	require.Equal(t, InteractionNeedsApproval, policy.CanInteract(vocab.PropertyCanReply, InteractionRequest{ActorID: policyTestStranger}))
	require.Equal(t, InteractionDenied, policy.CanInteract(vocab.PropertyCanLike, other))

	require.Equal(t, sliceof.String{policyTestAuthor}, interactionIDs([2]string{policyTestAuthor, ""}), "an array")
	require.Empty(t, interactionIDs(map[int]any{1: policyTestAuthor}), "a map without string keys")
	require.Equal(t, mapof.NewAny(), interactionMap(map[int]any{1: policyTestAuthor}), "a map without string keys")
	require.Equal(t, mapof.NewAny(), interactionMap(policyTestList{}), "an empty named list")
}
