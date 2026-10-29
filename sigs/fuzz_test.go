package sigs

import (
	"bytes"
	"crypto"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// FuzzParseSignature ensures ParseSignature never panics on an attacker-controlled Signature header,
// that a parsed Signature has every required field, and that it survives a String() round trip
func FuzzParseSignature(f *testing.F) {

	f.Add("")
	f.Add(`keyId="k",headers="host",signature="Y2FiYWIxNGRiZDk4ZA=="`)
	f.Add(`keyId="k"`)
	f.Add(`garbage`)
	f.Add(`signature="!!!"`)
	f.Add(`keyId="k",headers="host date",signature="AAAA",created="123",expires="456"`)
	f.Add(`keyId="k=v",algorithm="a\"b",headers="",signature="AAAA",created=-5`)
	f.Add(`headers="host",signature="AAAA"`)
	f.Add(`keyId="k",signature="AAAA"`)
	f.Add(`keyId="k",headers="host"`)

	f.Fuzz(func(t *testing.T, input string) {

		signature, err := ParseSignature(input)

		// A returned error is fine, as long as nothing panicked
		if err != nil {
			return
		}

		// RULE: A parsed Signature carries every field that ParseSignature requires
		require.NotEmpty(t, signature.KeyID)
		require.NotEmpty(t, signature.Headers)
		require.NotEmpty(t, signature.Signature)

		// Writing the Signature out and reading it back changes nothing. String() omits a
		// created or expires that is not positive, so those read back as zero.
		reparsed, err := ParseSignature(signature.String())
		require.NoError(t, err, signature.String())
		require.Equal(t, signature.KeyID, reparsed.KeyID)
		require.Equal(t, signature.Algorithm, reparsed.Algorithm)
		require.Equal(t, signature.Headers, reparsed.Headers)
		require.Equal(t, signature.Signature, reparsed.Signature)
		require.Equal(t, max(signature.Created, 0), reparsed.Created)
		require.Equal(t, max(signature.Expires, 0), reparsed.Expires)
	})
}

// FuzzVerifyDigest ensures VerifyDigest never panics on an attacker-controlled Digest header,
// and never accepts a non-empty header that lacks the body's real digest
func FuzzVerifyDigest(f *testing.F) {

	body := []byte("body")
	realSHA256 := DigestSHA256(body)
	realSHA512 := DigestSHA512(body)

	f.Add("")
	f.Add("SHA-256=abc")
	f.Add("SHA-256=,SHA-512=")
	f.Add("garbage")
	f.Add("MD5=x,SHA-256=y")
	f.Add("SHA-256=" + realSHA256)
	f.Add("sha512=" + realSHA512)
	f.Add("MD5=x, SHA-256=" + realSHA256)
	f.Add("SHA-256=" + realSHA256 + ",SHA-512=wrong")

	f.Fuzz(func(t *testing.T, digestHeader string) {

		request, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
		if err != nil {
			t.Skip()
		}
		request.Header.Set(FieldDigest, digestHeader)

		// An absent header is not checked here, so only a non-empty header can prove anything
		if err := VerifyDigest(request, crypto.SHA256, crypto.SHA512); err != nil || digestHeader == "" {
			return
		}

		// RULE: A header that verifies must carry one of the body's real digests
		require.True(t, strings.Contains(digestHeader, realSHA256) || strings.Contains(digestHeader, realSHA512), digestHeader)
	})
}

// FuzzDecodePublicPEM ensures DecodePublicPEM never panics on an arbitrary PEM string.
// The public key is fetched from a remote actor's profile, so it is attacker-controlled.
func FuzzDecodePublicPEM(f *testing.F) {

	f.Add("")
	f.Add("not pem at all")
	f.Add("-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----")
	f.Add("-----BEGIN RSA PUBLIC KEY-----\n\n-----END RSA PUBLIC KEY-----")
	f.Add("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----")

	f.Fuzz(func(_ *testing.T, input string) {
		// Must not panic. A returned error is fine; a valid key is fine.
		_, _ = DecodePublicPEM(input)
	})
}

// FuzzDecodePrivatePEM ensures DecodePrivatePEM never panics on an arbitrary PEM string.
func FuzzDecodePrivatePEM(f *testing.F) {

	f.Add("")
	f.Add("garbage")
	f.Add("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----")
	f.Add("-----BEGIN RSA PRIVATE KEY-----\n\n-----END RSA PRIVATE KEY-----")

	f.Fuzz(func(_ *testing.T, input string) {
		// Must not panic. A returned error is fine; a valid key is fine.
		_, _ = DecodePrivatePEM(input)
	})
}
