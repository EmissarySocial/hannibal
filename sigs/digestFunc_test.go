package sigs

import (
	"crypto"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetDigestName confirms the human-readable name for each supported hash,
// and the "unknown" fallback.
func TestGetDigestName(t *testing.T) {
	require.Equal(t, "SHA-256", getDigestName(crypto.SHA256))
	require.Equal(t, "SHA-512", getDigestName(crypto.SHA512))
	require.Equal(t, "unknown", getDigestName(crypto.MD5))
}

// TestGetDigestFunc confirms the DigestFunc selection for supported hashes and
// the error for unsupported ones.
func TestGetDigestFunc(t *testing.T) {

	fn, err := getDigestFunc(crypto.SHA256)
	require.Nil(t, err)
	require.NotNil(t, fn)
	require.Equal(t, DigestSHA256([]byte("abc")), fn([]byte("abc")))

	fn, err = getDigestFunc(crypto.SHA512)
	require.Nil(t, err)
	require.NotNil(t, fn)
	require.Equal(t, DigestSHA512([]byte("abc")), fn([]byte("abc")))

	// An unsupported algorithm returns an error and a nil function.
	fn, err = getDigestFunc(crypto.MD5)
	require.NotNil(t, err)
	require.Nil(t, fn)
}

// TestGetHashByName confirms case-insensitive name parsing and the
// default-to-SHA256 behavior for unknown names.
func TestGetHashByName(t *testing.T) {

	require.Equal(t, crypto.SHA256, getHashByName("sha-256"))
	require.Equal(t, crypto.SHA256, getHashByName("SHA256"))
	require.Equal(t, crypto.SHA512, getHashByName("sha-512"))
	require.Equal(t, crypto.SHA512, getHashByName("SHA512"))

	// Unknown names default to SHA-256 (documented behavior).
	require.Equal(t, crypto.SHA256, getHashByName("md5"))
	require.Equal(t, crypto.SHA256, getHashByName(""))
}

// TestLookupHashByName confirms the lookup helper reports whether a name was
// recognized, which is what lets VerifyDigest skip unknown algorithms.
func TestLookupHashByName(t *testing.T) {

	hash, ok := lookupHashByName("sha-256")
	require.True(t, ok)
	require.Equal(t, crypto.SHA256, hash)

	hash, ok = lookupHashByName("SHA-512")
	require.True(t, ok)
	require.Equal(t, crypto.SHA512, hash)

	// Unknown names are reported as not-ok, NOT silently mapped to SHA-256.
	_, ok = lookupHashByName("md5")
	require.False(t, ok)

	_, ok = lookupHashByName("")
	require.False(t, ok)
}

// TestGetDigestFuncByName confirms that recognized names return a function and
// unknown names return an error (so callers can skip them).
func TestGetDigestFuncByName(t *testing.T) {

	fn, err := getDigestFuncByName("SHA-256")
	require.Nil(t, err)
	require.NotNil(t, fn)
	require.Equal(t, DigestSHA256([]byte("abc")), fn([]byte("abc")))

	// Unknown algorithm -> error, so VerifyDigest skips it instead of treating
	// it as SHA-256.
	fn, err = getDigestFuncByName("MD5")
	require.NotNil(t, err)
	require.Nil(t, fn)
}
