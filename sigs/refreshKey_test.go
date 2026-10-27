package sigs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/benpate/derp"
	"github.com/stretchr/testify/require"
)

// refreshKeyID is the keyId used by the fixtures below.
const refreshKeyID = "https://remote.example/@alice#main-key"

// countingFinder is a PublicKeyFinder that records how many times it was called.
type countingFinder struct {
	publicKeyPEM string // Key handed to the caller
	calls        int    // Number of times this finder ran
	err          error  // If set, every lookup fails with this error
}

// find implements PublicKeyFinder.  It serves the same key to every keyID, because these tests turn
// on WHETHER a finder ran, never on which key it was asked for.
func (finder *countingFinder) find(_ string) (string, error) {

	finder.calls++

	if finder.err != nil {
		return "", finder.err
	}

	return finder.publicKeyPEM, nil
}

// newRSARequest returns a GET request signed with a fresh RSA key, plus that key's PEM.
func newRSARequest(t *testing.T) (*http.Request, string) {

	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := httptest.NewRequest(http.MethodGet, "https://local.example/@bob/pub/objects/123", nil)
	require.Nil(t, Sign(request, refreshKeyID, privateKey))

	return request, EncodePublicPEM(privateKey)
}

// TestRefreshKey_NotCalledOnSuccess is the cost argument for the whole feature: the common case must
// not pay for it.
func TestRefreshKey_NotCalledOnSuccess(t *testing.T) {

	request, publicKeyPEM := newRSARequest(t)

	primary := &countingFinder{publicKeyPEM: publicKeyPEM}
	refresh := &countingFinder{publicKeyPEM: publicKeyPEM}

	signature, err := Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.Nil(t, err)
	require.Equal(t, refreshKeyID, signature.KeyID)
	require.Equal(t, 1, primary.calls)
	require.Equal(t, 0, refresh.calls)
}

// TestRefreshKey_RepairsRotation is the property the option exists for: a peer that has rotated its
// key is verified on the first delivery that fails, rather than after a window of rejected traffic.
func TestRefreshKey_RepairsRotation(t *testing.T) {

	request, rotatedPEM := newRSARequest(t)

	// The primary finder still holds the peer's PREVIOUS key, so the first attempt cannot verify
	_, stalePEM := newRSARequest(t)

	primary := &countingFinder{publicKeyPEM: stalePEM}
	refresh := &countingFinder{publicKeyPEM: rotatedPEM}

	signature, err := Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.Nil(t, err)
	require.Equal(t, refreshKeyID, signature.KeyID)
	require.Equal(t, 1, refresh.calls, "exactly one refresh, and no third attempt")
}

// TestRefreshKey_RepairsRotationOnSignedPost is the same repair on the path that actually matters --
// a POST to an inbox, where the signature covers a body Digest.
func TestRefreshKey_RepairsRotationOnSignedPost(t *testing.T) {

	// The Digest is verified ONCE, before the signature is even parsed, so the second attempt never
	// re-reads the body.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	body := `{"type":"Create","actor":"https://remote.example/@alice"}`
	request := httptest.NewRequest(http.MethodPost, "https://local.example/@bob/pub/inbox", strings.NewReader(body))
	require.Nil(t, Sign(request, refreshKeyID, privateKey))

	_, stalePEM := newRSARequest(t)

	primary := &countingFinder{publicKeyPEM: stalePEM}
	refresh := &countingFinder{publicKeyPEM: EncodePublicPEM(privateKey)}

	_, err = Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.Nil(t, err)
	require.Equal(t, 1, refresh.calls)
}

// TestRefreshKey_RepairsRotation_ECDSA covers the other branch of verifySignature, since a rotation
// that only worked for RSA keys would be a silent hole for every ECDSA peer.
func TestRefreshKey_RepairsRotation_ECDSA(t *testing.T) {

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)

	request := httptest.NewRequest(http.MethodGet, "https://local.example/@bob/pub/objects/123", nil)
	require.Nil(t, Sign(request, refreshKeyID, privateKey))

	stalePrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)

	primary := &countingFinder{publicKeyPEM: encodeECDSAPublicPEM(t, &stalePrivateKey.PublicKey)}
	refresh := &countingFinder{publicKeyPEM: encodeECDSAPublicPEM(t, &privateKey.PublicKey)}

	_, err = Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.Nil(t, err)
	require.Equal(t, 1, refresh.calls)
}

// TestRefreshKey_UnchangedKeyIsNotReverified pins the second half of the rule: the refresh happens,
// but an unchanged key ends the attempt instead of paying for another crypto check.
func TestRefreshKey_UnchangedKeyIsNotReverified(t *testing.T) {

	request, _ := newRSARequest(t)

	// A key that does not match, from an origin that keeps returning that same key
	_, wrongPEM := newRSARequest(t)

	primary := &countingFinder{publicKeyPEM: wrongPEM}
	refresh := &countingFinder{publicKeyPEM: wrongPEM}

	_, err := Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.NotNil(t, err)
	require.Equal(t, 1, refresh.calls)

	// The ORIGINAL error is returned. A second hash loop would have wrapped it.
	require.NotContains(t, derp.Message(err), "refreshed key")
}

// TestRefreshKey_FailedRefreshReturnsOriginalError covers the peer we cannot reach: the refresh error
// is deliberately dropped, because a forged keyID makes this path fail as a matter of course.
func TestRefreshKey_FailedRefreshReturnsOriginalError(t *testing.T) {

	request, _ := newRSARequest(t)
	_, wrongPEM := newRSARequest(t)

	primary := &countingFinder{publicKeyPEM: wrongPEM}
	refresh := &countingFinder{err: derp.Internal("test", "Remote host unreachable")}

	_, err := Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.NotNil(t, err)
	require.Equal(t, 1, refresh.calls)
	require.NotContains(t, derp.Message(err), "Remote host unreachable")
	require.Equal(t, "No valid signatures found", derp.Message(err), "the first attempt's error is returned unwrapped")
}

// TestRefreshKey_NilBehavesAsBefore confirms that the option is genuinely additive.
func TestRefreshKey_NilBehavesAsBefore(t *testing.T) {

	request, publicKeyPEM := newRSARequest(t)
	primary := &countingFinder{publicKeyPEM: publicKeyPEM}

	_, err := Verify(request, primary.find)
	require.Nil(t, err)

	// ...and a failure with no RefreshKey is still final
	_, wrongPEM := newRSARequest(t)
	wrongFinder := &countingFinder{publicKeyPEM: wrongPEM}

	_, err = Verify(request, wrongFinder.find)
	require.NotNil(t, err)
}

// TestRefreshKey_NotCalledOnUnparseableSignature is the first of four early-return cases.  Each one
// is a separate path back out of Verify, and each one must cost zero lookups -- a refresh that fired
// before the hash loop would let anyone provoke an outbound fetch at a host of their choosing.
func TestRefreshKey_NotCalledOnUnparseableSignature(t *testing.T) {

	request := httptest.NewRequest(http.MethodGet, "https://local.example/@bob/pub/objects/123", nil)
	request.Header.Set("Signature", "this-is-not-a-signature")

	primary := &countingFinder{publicKeyPEM: "irrelevant"}
	refresh := &countingFinder{publicKeyPEM: "irrelevant"}

	_, err := Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.NotNil(t, err)
	require.Equal(t, 0, primary.calls, "an unparseable signature names no keyID to look up")
	require.Equal(t, 0, refresh.calls)
}

// TestRefreshKey_NotCalledOnExpiredDate covers the replay guard.
func TestRefreshKey_NotCalledOnExpiredDate(t *testing.T) {

	request, publicKeyPEM := newRSARequest(t)
	request.Header.Set(FieldDate, "Mon, 04 Sep 2023 21:17:36 GMT")

	primary := &countingFinder{publicKeyPEM: publicKeyPEM}
	refresh := &countingFinder{publicKeyPEM: publicKeyPEM}

	_, err := Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.NotNil(t, err)
	require.Equal(t, 0, primary.calls, "an expired request is refused before any key is fetched")
	require.Equal(t, 0, refresh.calls)
}

// TestRefreshKey_NotCalledOnBadDigest covers a POST whose body does not match the Digest it carries.
func TestRefreshKey_NotCalledOnBadDigest(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	request := httptest.NewRequest(http.MethodPost, "https://local.example/@bob/pub/inbox", strings.NewReader(`{"type":"Create"}`))
	require.Nil(t, Sign(request, refreshKeyID, privateKey))

	// Corrupt the Digest AFTER signing, so it no longer describes the body
	request.Header.Set(FieldDigest, "SHA-256=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")

	primary := &countingFinder{publicKeyPEM: EncodePublicPEM(privateKey)}
	refresh := &countingFinder{publicKeyPEM: EncodePublicPEM(privateKey)}

	_, err = Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.NotNil(t, err)
	require.Equal(t, 0, primary.calls, "the digest is checked before any key is fetched")
	require.Equal(t, 0, refresh.calls)
}

// TestRefreshKey_NotCalledWhenPrimaryFinderFails is the case that keeps a broken lookup from being
// retried against the same broken source.
func TestRefreshKey_NotCalledWhenPrimaryFinderFails(t *testing.T) {

	request, _ := newRSARequest(t)

	primary := &countingFinder{err: derp.Internal("test", "Remote host unreachable")}
	refresh := &countingFinder{publicKeyPEM: "irrelevant"}

	_, err := Verify(request, primary.find, WithRefreshKey(refresh.find))

	require.NotNil(t, err)
	require.Equal(t, 1, primary.calls)
	require.Equal(t, 0, refresh.calls)
}
