package sigs

import (
	"bytes"
	"crypto"
	"net/http"
	"testing"
)

// FuzzParseSignature ensures ParseSignature never panics on arbitrary input.
// The Signature header is attacker-controlled, so robust parsing is critical.
func FuzzParseSignature(f *testing.F) {

	f.Add("")
	f.Add(`keyId="k",headers="host",signature="Y2FiYWIxNGRiZDk4ZA=="`)
	f.Add(`keyId="k"`)
	f.Add(`garbage`)
	f.Add(`signature="!!!"`)
	f.Add(`keyId="k",headers="host date",signature="AAAA",created="123",expires="456"`)

	f.Fuzz(func(_ *testing.T, input string) {
		// Must not panic. A returned error is fine; a valid Signature is fine.
		_, _ = ParseSignature(input)
	})
}

// FuzzVerifyDigest ensures VerifyDigest never panics on an arbitrary Digest
// header. The header is attacker-controlled.
func FuzzVerifyDigest(f *testing.F) {

	f.Add("")
	f.Add("SHA-256=abc")
	f.Add("SHA-256=,SHA-512=")
	f.Add("garbage")
	f.Add("MD5=x,SHA-256=y")

	f.Fuzz(func(t *testing.T, digestHeader string) {

		request, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader([]byte("body")))
		if err != nil {
			t.Skip()
		}
		request.Header.Set(FieldDigest, digestHeader)

		// Must not panic. Any error/no-error result is acceptable.
		_ = VerifyDigest(request, crypto.SHA256, crypto.SHA512)
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
