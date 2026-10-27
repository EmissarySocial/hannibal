package vocab

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestContextTypeGoToSocial pins that every GoToSocial term is declared under the gts: namespace, as an IRI
func TestContextTypeGoToSocial(t *testing.T) {

	require.Equal(t, "https://gotosocial.org/ns#", ContextTypeGoToSocial["gts"])

	for _, term := range []string{PropertyInteractionPolicy, PropertyCanLike, PropertyCanReply, PropertyCanAnnounce, PropertyCanQuote, PropertyAutomaticApproval, PropertyManualApproval, PropertyInteractingObject, PropertyInteractionTarget} {
		require.Equal(t, map[string]any{"@id": "gts:" + term, "@type": "@id"}, ContextTypeGoToSocial[term], term)
	}
}

// TestContextTypeQuotePosts pins FEP-044f's terms and their IRIs
func TestContextTypeQuotePosts(t *testing.T) {

	require.Equal(t, map[string]any{"@id": "https://w3id.org/fep/044f#quote", "@type": "@id"}, ContextTypeQuotePosts[PropertyQuote])
	require.Equal(t, map[string]any{"@id": "https://w3id.org/fep/044f#quoteAuthorization", "@type": "@id"}, ContextTypeQuotePosts[PropertyQuoteAuthorization])
	require.Equal(t, "https://w3id.org/fep/044f#QuoteRequest", ContextTypeQuotePosts[ActivityTypeQuoteRequest])
	require.Equal(t, "https://w3id.org/fep/044f#QuoteAuthorization", ContextTypeQuotePosts[ObjectTypeQuoteAuthorization])
}
