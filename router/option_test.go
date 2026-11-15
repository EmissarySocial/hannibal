package router

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewReceiveConfig_Defaults confirms the default config ships with a
// validator chain (HTTP signature + deleted-object checks).
func TestNewReceiveConfig_Defaults(t *testing.T) {

	config := NewReceiveConfig()
	require.NotEmpty(t, config.Validators)

	// The default chain must include an HTTPSig validator.
	hasHTTPSig := false
	for _, v := range config.Validators {
		if _, ok := v.(validator.HTTPSig); ok {
			hasHTTPSig = true
		}
	}
	assert.True(t, hasHTTPSig, "default chain should include an HTTPSig validator")
}

// TestWithValidators confirms the option replaces the entire validator chain.
func TestWithValidators(t *testing.T) {

	custom := stubValidator{validator.ResultValid}
	config := NewReceiveConfig(WithValidators(custom))

	require.Len(t, config.Validators, 1)
	assert.Equal(t, custom, config.Validators[0])
}

// TestWithPublicKeyFinder confirms the option swaps the default HTTPSig validator
// for one bound to the provided key finder, leaving the rest of the chain intact.
func TestWithPublicKeyFinder(t *testing.T) {

	defaultCount := len(NewReceiveConfig().Validators)

	keyFinder := func(_ string) (string, error) { return "", nil }
	config := NewReceiveConfig(WithPublicKeyFinder(keyFinder))

	// The chain length is unchanged -- the HTTPSig validator is replaced in place.
	assert.Len(t, config.Validators, defaultCount)

	// An HTTPSig validator is still present.
	hasHTTPSig := false
	for _, v := range config.Validators {
		if _, ok := v.(validator.HTTPSig); ok {
			hasHTTPSig = true
		}
	}
	assert.True(t, hasHTTPSig)
}

// signedActivity returns a signed POST of an activity from the given actor, and the finder for its key
func signedActivity(t *testing.T, actorID string, body string) (*http.Request, sigs.PublicKeyFinder) {

	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", strings.NewReader(body))
	require.NoError(t, sigs.Sign(request, actorID+"#main-key", privateKey))

	return request, func(string) (string, error) {
		return sigs.EncodePublicPEM(privateKey), nil
	}
}

// TestWithPublicKeyFinder_AnyOrder confirms the key finder reaches a validator chain that a
// later option installs, and that it keeps each validator's own verifier options
func TestWithPublicKeyFinder_AnyOrder(t *testing.T) {

	const actorID = "https://example.com/users/alice"
	body := `{"type":"Follow","actor":"` + actorID + `","object":"https://example.com/users/bob"}`

	failing := func(string) (string, error) {
		return "", derp.Internal("test", "This finder must be replaced")
	}

	// The custom chain is installed AFTER the key finder is named
	request, keyFinder := signedActivity(t, actorID, body)

	_, err := ReceiveRequest(request, actorClient{actorID: actorID},
		WithPublicKeyFinder(keyFinder),
		WithValidators(validator.NewHTTPSig(failing)),
	)
	require.NoError(t, err)

	// RULE: The validator's own options survive. This one refuses every signature hash, so
	// a request that verified above must now fail.
	request, keyFinder = signedActivity(t, actorID, body)

	_, err = ReceiveRequest(request, actorClient{actorID: actorID},
		WithValidators(validator.NewHTTPSig(failing, sigs.VerifierSignatureHashes())),
		WithPublicKeyFinder(keyFinder),
	)
	require.Error(t, err)
}

// TestWithMaxBodySize_ReachesTheVerifier confirms a signed body over the default limit is
// accepted when the caller raises the limit
func TestWithMaxBodySize_ReachesTheVerifier(t *testing.T) {

	const actorID = "https://example.com/users/alice"
	body := `{"type":"Follow","actor":"` + actorID + `","object":"https://example.com/users/bob","summary":"` + strings.Repeat("a", 1<<20) + `"}`

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyFinder := func(string) (string, error) {
		return sigs.EncodePublicPEM(privateKey), nil
	}

	// Sign by hand: the signer's own Digest step stops at the default limit
	newRequest := func() *http.Request {

		request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", strings.NewReader(body))
		sum := sha256.Sum256([]byte(body))
		request.Header.Set("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(sum[:]))

		signer := sigs.NewSigner(actorID+"#main-key", privateKey, sigs.SignerFields(sigs.FieldRequestTarget, sigs.FieldHost, sigs.FieldDate))
		signature, err := signer.MakeSignature(request)
		require.NoError(t, err)
		request.Header.Set("Signature", signature.String())

		return request
	}

	options := []Option{
		WithPublicKeyFinder(keyFinder),
		WithValidators(validator.NewHTTPSig(nil, sigs.VerifierFields(sigs.FieldRequestTarget, sigs.FieldHost, sigs.FieldDate))),
	}

	_, err = ReceiveRequest(newRequest(), actorClient{actorID: actorID}, append(options, WithMaxBodySize(2<<20))...)
	require.NoError(t, err)

	_, err = ReceiveRequest(newRequest(), actorClient{actorID: actorID}, options...)
	require.Error(t, err, "the default limit still applies")
}

// actorClient is a streams.Client that knows one actor
type actorClient struct {
	actorID string
}

// SetRootClient is a NOOP for this stub
func (actorClient) SetRootClient(streams.Client) {}

// Save is a NOOP for this stub
func (actorClient) Save(streams.Document) error { return nil }

// Delete is a NOOP for this stub
func (actorClient) Delete(string) error { return nil }

// Load returns the actor for its own ID, and NotFound for anything else
func (client actorClient) Load(url string, _ ...any) (streams.Document, error) {

	if url != client.actorID {
		return streams.NilDocument(), derp.NotFound("test", "Document not found", url)
	}

	return streams.NewDocument(map[string]any{"id": client.actorID, "type": "Person"}, streams.WithClient(client)), nil
}
