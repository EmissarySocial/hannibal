package sigs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/require"
)

/******************************************
 * Signer Options
 ******************************************/

// TestSignerOptions confirms each SignerOption sets the matching field.
func TestSignerOptions(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	signer := NewSigner("key-id", privateKey,
		SignerFields(FieldHost, FieldDate),
		SignerSignatureHash(crypto.SHA512),
		SignerBodyDigest(crypto.SHA512),
		SignerCreated(1700000000),
		SignerExpires(1700003600),
	)

	require.Equal(t, []string{FieldHost, FieldDate}, signer.Fields)
	require.Equal(t, crypto.SHA512, signer.SignatureHash)
	require.Equal(t, crypto.SHA512, signer.BodyDigest)
	require.Equal(t, int64(1700000000), signer.Created)
	require.Equal(t, int64(1700003600), signer.Expires)
}

/******************************************
 * Verifier Options
 ******************************************/

// TestVerifierOptions confirms each VerifierOption sets the matching field.
func TestVerifierOptions(t *testing.T) {

	verifier := NewVerifier(
		VerifierFields(FieldHost, FieldDate),
		VerifierBodyDigests(crypto.SHA512),
		VerifierSignatureHashes(crypto.SHA512),
		VerifierTimeout(99),
	)

	require.Equal(t, []string{FieldHost, FieldDate}, verifier.Fields)
	require.Equal(t, []crypto.Hash{crypto.SHA512}, verifier.BodyDigests)
	require.Equal(t, []crypto.Hash{crypto.SHA512}, verifier.SignatureHashes)
	require.Equal(t, 99, verifier.Timeout)
}

// TestVerifierIgnoreOptions confirms the "ignore" options relax their checks.
func TestVerifierIgnoreOptions(t *testing.T) {

	verifier := NewVerifier(
		VerifierIgnoreTimeout(),
		VerifierIgnoreBodyDigest(),
	)

	require.Equal(t, 0, verifier.Timeout, "IgnoreTimeout should zero the timeout")
	require.False(t, verifier.CheckDigest, "IgnoreBodyDigest should disable the digest check")
}

/******************************************
 * Certificate / Algorithm Helpers
 ******************************************/

// TestEncodePrivatePEM_RoundTrip confirms a private key survives an
// encode/decode round trip.
func TestEncodePrivatePEM_RoundTrip(t *testing.T) {

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	pemString := EncodePrivatePEM(privateKey)
	require.NotEmpty(t, pemString)

	decoded, err := DecodePrivatePEM(pemString)
	require.Nil(t, err)

	// The decoded key must match the original.
	require.True(t, privateKey.Equal(decoded))
}

// TestDecodePrivatePEM_Reject confirms undecodable input is rejected.
func TestDecodePrivatePEM_Reject(t *testing.T) {

	_, err := DecodePrivatePEM("not a pem")
	require.NotNil(t, err)

	// Valid PEM framing but an unrecognized block type.
	_, err = DecodePrivatePEM("-----BEGIN NONSENSE-----\nAAAA\n-----END NONSENSE-----")
	require.NotNil(t, err)
}

// TestDecodePublicPEM_Reject confirms undecodable input is rejected.
func TestDecodePublicPEM_Reject(t *testing.T) {

	_, err := DecodePublicPEM("not a pem")
	require.NotNil(t, err)
}

// TestGetAlgorithmName confirms the algorithm name is derived from the key type
// and hash, falling back to hs2019 for unknowns.
func TestGetAlgorithmName(t *testing.T) {

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)

	require.Equal(t, "rsa-sha256", getAlgorithmName(rsaKey, crypto.SHA256))
	require.Equal(t, "rsa-sha512", getAlgorithmName(rsaKey, crypto.SHA512))
	require.Equal(t, "ecdsa-sha256", getAlgorithmName(ecdsaKey, crypto.SHA256))
	require.Equal(t, "ecdsa-sha512", getAlgorithmName(ecdsaKey, crypto.SHA512))

	// Unknown hash -> hs2019 fallback.
	require.Equal(t, Algorithm_HS2019, getAlgorithmName(rsaKey, crypto.MD5))

	// Unknown key type -> hs2019 fallback.
	require.Equal(t, Algorithm_HS2019, getAlgorithmName("not-a-key", crypto.SHA256))
}

/******************************************
 * Mock Verifier
 ******************************************/

// TestMockVerifier confirms the mock honors its configured success flag.
func TestMockVerifier(t *testing.T) {

	keyFinder := func(_ string) (string, error) { return "", nil }

	success := NewMockVerifier("mock-key", true)
	signature, err := success.Verify(nil, keyFinder)
	require.Nil(t, err)
	require.Equal(t, "mock-key", signature.KeyID)

	failure := NewMockVerifier("mock-key", false)
	_, err = failure.Verify(nil, keyFinder)
	require.NotNil(t, err)
}
