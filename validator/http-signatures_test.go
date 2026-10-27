package validator

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/hannibal/streams"
	"github.com/stretchr/testify/require"
)

// signedRequestForActor builds and signs a POST request, returning the request
// and a key finder that serves the signing key. keyID is the public key ID used
// as the signature's keyID (and therefore its ActorID once the fragment is
// stripped).
func signedRequestForActor(t *testing.T, keyID string) (*http.Request, sigs.PublicKeyFinder) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox",
		bytes.NewReader([]byte(`{"hello":"world"}`)))

	// Sign supplies the "Date" header itself when it is absent.
	require.NoError(t, sigs.Sign(request, keyID, privateKey))

	keyFinder := func(_ string) (string, error) {
		return sigs.EncodePublicPEM(privateKey), nil
	}

	return request, keyFinder
}

// TestHTTPSig_NoSignature confirms an unsigned request yields Unknown (the
// validator abstains so other validators in the chain can run).
func TestHTTPSig_NoSignature(t *testing.T) {

	v := NewHTTPSig(func(string) (string, error) { return "", nil })

	request := httptest.NewRequest(http.MethodPost, "https://example.com/inbox", nil)
	activity := streams.NewDocument(map[string]any{})

	require.Equal(t, ResultUnknown, v.Validate(request, &activity))
}

// TestHTTPSig_DefaultKeyFinder confirms that, when no key finder is supplied,
// the validator falls back to its default finder, which loads the signing actor
// from its origin server. Offline, that load fails, so a signed request whose
// actor cannot be resolved is rejected as Invalid. This exercises the
// defaultKeyFinder construction and its error path.
func TestHTTPSig_DefaultKeyFinder(t *testing.T) {

	actorID := "https://offline.example.com/users/alice"
	request, _ := signedRequestForActor(t, actorID+"#main-key")

	// Passing nil forces the validator to build and use defaultKeyFinder.
	v := NewHTTPSig(nil)
	activity := actorDocument(actorID)

	require.Equal(t, ResultInvalid, v.Validate(request, &activity))
}

// TestHTTPSig_Valid confirms a correctly signed request whose signature actor
// matches the activity actor is Valid.
func TestHTTPSig_Valid(t *testing.T) {

	actorID := "https://example.com/users/alice"
	request, keyFinder := signedRequestForActor(t, actorID+"#main-key")

	v := NewHTTPSig(keyFinder)
	activity := actorDocument(actorID)

	require.Equal(t, ResultValid, v.Validate(request, &activity))
}

// TestHTTPSig_ActorMismatch is the key security test: a request with a perfectly
// valid signature, but whose signing actor does NOT match the activity's actor,
// must be rejected. This prevents one actor from forging activities on behalf of
// another.
func TestHTTPSig_ActorMismatch(t *testing.T) {

	// The request is signed by alice...
	request, keyFinder := signedRequestForActor(t, "https://example.com/users/alice#main-key")

	v := NewHTTPSig(keyFinder)

	// ...but the activity claims to be from eve.
	activity := actorDocument("https://example.com/users/eve")

	require.Equal(t, ResultInvalid, v.Validate(request, &activity),
		"a valid signature whose actor does not match the activity actor must be rejected")
}

// TestHTTPSig_BadSignature confirms a tampered signature is rejected.
func TestHTTPSig_BadSignature(t *testing.T) {

	actorID := "https://example.com/users/alice"
	request, keyFinder := signedRequestForActor(t, actorID+"#main-key")

	// Corrupt the signature header after signing.
	signature, err := sigs.ParseSignature(sigs.GetSignature(request))
	require.NoError(t, err)
	signature.Signature[0] ^= 0xFF
	request.Header.Set("Signature", signature.String())

	v := NewHTTPSig(keyFinder)
	activity := actorDocument(actorID)

	require.Equal(t, ResultInvalid, v.Validate(request, &activity))
}

// TestHTTPSig_WrongKey confirms a signature that verifies against a different key
// than the one that signed it is rejected.
func TestHTTPSig_WrongKey(t *testing.T) {

	actorID := "https://example.com/users/alice"
	request, _ := signedRequestForActor(t, actorID+"#main-key")

	// The key finder returns an unrelated key.
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	wrongFinder := func(string) (string, error) {
		return sigs.EncodePublicPEM(otherKey), nil
	}

	v := NewHTTPSig(wrongFinder)
	activity := actorDocument(actorID)

	require.Equal(t, ResultInvalid, v.Validate(request, &activity))
}

// TestHTTPSig_OptionsReachVerifier confirms that options given to NewHTTPSig are actually forwarded
// to sigs.Verify, rather than being accepted and quietly dropped.
func TestHTTPSig_OptionsReachVerifier(t *testing.T) {

	actorID := "https://example.com/users/alice"
	request, keyFinder := signedRequestForActor(t, actorID+"#main-key")

	// VerifierFields demands a header that this request does not sign, so a validator that honors the
	// option must reject a request that it would otherwise accept.
	v := NewHTTPSig(keyFinder, sigs.VerifierFields("(request-target)", "host", "date", "digest", "x-not-signed"))
	activity := actorDocument(actorID)

	require.Equal(t, ResultInvalid, v.Validate(request, &activity))
}

// TestHTTPSig_RefreshKeyRepairsRotation is the whole point of forwarding options: a peer that has
// rotated its key is accepted through the validator, without the caller re-implementing the retry.
func TestHTTPSig_RefreshKeyRepairsRotation(t *testing.T) {

	actorID := "https://example.com/users/alice"
	request, rotatedFinder := signedRequestForActor(t, actorID+"#main-key")

	// The primary finder is holding a stale key, as a cache would be after the peer rotated
	staleKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	staleFinder := func(string) (string, error) {
		return sigs.EncodePublicPEM(staleKey), nil
	}

	v := NewHTTPSig(staleFinder, sigs.WithRefreshKey(rotatedFinder))
	activity := actorDocument(actorID)

	require.Equal(t, ResultValid, v.Validate(request, &activity))
}

// TestHTTPSig_RefreshKeyStillChecksActor confirms the two rules compose in the right order: a
// refreshed key gets a signature verified, and the actor-match rule then rejects it anyway.
func TestHTTPSig_RefreshKeyStillChecksActor(t *testing.T) {

	// This is the rule that Emissary had to duplicate while the retry lived downstream. A refresh
	// that skipped it would let alice's rotated key authenticate an activity attributed to eve.
	request, rotatedFinder := signedRequestForActor(t, "https://example.com/users/alice#main-key")

	staleKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	staleFinder := func(string) (string, error) {
		return sigs.EncodePublicPEM(staleKey), nil
	}

	v := NewHTTPSig(staleFinder, sigs.WithRefreshKey(rotatedFinder))
	activity := actorDocument("https://example.com/users/eve")

	require.Equal(t, ResultInvalid, v.Validate(request, &activity))
}
