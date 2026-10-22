package streams

import (
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/require"
)

// TestInteractionRequest_IsMemberOfAny pins which IRIs reach IsMember, and that the first match ends the search
func TestInteractionRequest_IsMemberOfAny(t *testing.T) {

	const otherCollection = "https://social.example/users/author/following"

	asked := sliceof.NewString()
	request := InteractionRequest{
		ActorID:  policyTestStranger,
		AuthorID: policyTestAuthor,
		IsMember: func(collectionID string) bool {
			asked = append(asked, collectionID)
			return collectionID == policyTestFollowers
		},
	}

	ids := sliceof.String{vocab.NamespaceActivityStreamsPublic, vocab.NamespaceASPublic, vocab.NamespacePublic, policyTestStranger, policyTestAuthor, otherCollection, policyTestFollowers, "https://never.example/asked"}

	require.True(t, request.isMemberOfAny(ids))
	require.Equal(t, sliceof.String{otherCollection, policyTestFollowers}, asked, "Public and the object's own actors are skipped, and the search stops at the first match")

	asked = sliceof.NewString()
	require.False(t, request.isMemberOfAny(sliceof.String{otherCollection}))
	require.Equal(t, sliceof.String{otherCollection}, asked)

	require.False(t, request.isMemberOfAny(sliceof.NewString()), "an empty list")

	request.IsMember = nil
	require.False(t, request.isMemberOfAny(ids), "no IsMember means no memberships")
}
