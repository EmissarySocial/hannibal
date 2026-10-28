package sigs

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSignRequest confirms Sign sets the expected SHA-256 Digest and a Signature that verifies
// against the signing key.
func TestSignRequest(t *testing.T) {

	body := []byte("This is the body of the request")

	request, err := http.NewRequest("GET", "http://example.com/something?test=true", bytes.NewReader(body))
	require.Nil(t, err)
	request.Header.Set("Content-Type", "text/plain")

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.Nil(t, err)

	err = Sign(request, "test-key", privateKey)
	require.Nil(t, err)

	require.Equal(t, "SHA-256=65F8+S1Bg7oPQS/fIxVg4x7PoLWnOxWlGMFB/hafojg=", request.Header.Get("Digest"))
	require.NotEmpty(t, request.Header.Get("Signature"))

	// The Signature must verify against the key that made it
	_, err = Verify(request, rsaKeyFinder(privateKey))
	require.Nil(t, err)
}

// TestMakePlaintext confirms makePlaintext lists each field in order, leaving unset headers empty
func TestMakePlaintext(t *testing.T) {

	body := []byte("This is the body of the request")

	request, err := http.NewRequest("GET", "http://example.com/something?test=true", bytes.NewReader(body))
	require.Nil(t, err)
	request.Header.Set("Content-Type", "text/plain")

	signature := NewSignature()
	result := makePlaintext(request, signature, FieldRequestTarget, FieldHost, FieldDate, FieldDigest)
	expected := removeTabs(
		`(request-target): get /something?test=true
		host: example.com
		date: 
		digest: `)
	require.Equal(t, expected, result)
}

// TestMakePlaintext_Alternate confirms makePlaintext lowercases a mixed-case header name and reads its value
func TestMakePlaintext_Alternate(t *testing.T) {

	bodyReader := strings.NewReader("This is the body of the request")

	request, err := http.NewRequest("GET", "http://example.com/something?test=true", bodyReader)
	require.Nil(t, err)
	request.Header.Set("Content-Type", "text/plain")

	signature := NewSignature()
	result := makePlaintext(request, signature, FieldRequestTarget, FieldHost, "Content-Type")
	expected := removeTabs(
		`(request-target): get /something?test=true
		host: example.com
		content-type: text/plain`)

	require.Equal(t, expected, result)
}

// TestMakeSignatureHash_SHA256 pins makeSignatureHash's SHA-256 output for a known string
func TestMakeSignatureHash_SHA256(t *testing.T) {
	result, err := makeSignatureHash("This is digest-able", crypto.SHA256)
	require.Nil(t, err)

	actual := base64.StdEncoding.EncodeToString(result)
	require.Equal(t, "jlBmJDmZdMjhLZga/ZjDrlloKd5lukG9S0lu/f7Xc64=", actual)
}

// TestMakeSignatureHash_SHA512 pins makeSignatureHash's SHA-512 output for a known string
func TestMakeSignatureHash_SHA512(t *testing.T) {
	result, err := makeSignatureHash("This is digest-able", crypto.SHA512)
	require.Nil(t, err)

	actual := base64.StdEncoding.EncodeToString(result)
	require.Equal(t, "s2JJ/rYbVQTrkNR440jq+wuNk9ktJgvmVSDq805iC0EP4ONQPwfvuQK0yR/YuX7riJtNRwxMq6R1GL8W7A5vzg==", actual)
}

// TestMakeSignatureHash_Unsupported confirms makeSignatureHash rejects a hash other than SHA-256 or SHA-512
func TestMakeSignatureHash_Unsupported(t *testing.T) {
	result, err := makeSignatureHash("This is digest-able", crypto.BLAKE2b_256)
	require.NotNil(t, err)
	require.Nil(t, result)
}

// TestGetPathAndQuery confirms getPathAndQuery defaults an empty path to "/" and keeps the query
func TestGetPathAndQuery(t *testing.T) {
	url, _ := url.Parse("http://example.com")
	require.Equal(t, "/", getPathAndQuery(url))

	url, _ = url.Parse("http://example.com/")
	require.Equal(t, "/", getPathAndQuery(url))

	url, _ = url.Parse("http://example.com/something")
	require.Equal(t, "/something", getPathAndQuery(url))

	url, _ = url.Parse("http://example.com/something?test=true")
	require.Equal(t, "/something?test=true", getPathAndQuery(url))
}

// removeTabs strips every tab from s, so test fixtures can be indented in the source
func removeTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "")
}
