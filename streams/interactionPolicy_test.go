package streams

import (
	"encoding/json"
	"testing"

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

// TestDocument_InteractionPolicy pins that the accessor reads an inline policy, and loads one published as an IRI
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
	require.Equal(t, sliceof.String{policyTestAuthor}, linked.InteractionPolicy().AutomaticApproval(vocab.PropertyCanReply))

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
