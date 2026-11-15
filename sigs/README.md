# Hannibal / sigs

Common-Sense HTTP Signatures

<img src="https://github.com/benpate/hannibal/raw/main/meta/sigs.jpg" style="width:100%; display:block; margin-bottom:20px;" alt="Oil painting titled: Signers of the Constitution, by Thomas Pritchard Rossiter (1817-1871)"/>


[![Go Reference](https://pkg.go.dev/badge/github.com/benpate/hannibal/sigs.svg)](https://pkg.go.dev/github.com/benpate/hannibal/sigs)
[![Build Status](https://img.shields.io/github/actions/workflow/status/benpate/hannibal/go.yml?branch=main)](https://github.com/benpate/hannibal/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/benpate/hannibal?style=flat-square)](https://goreportcard.com/report/github.com/benpate/hannibal)
[![Codecov](https://img.shields.io/codecov/c/github/benpate/hannibal/sigs.svg?style=flat-square)](https://codecov.io/gh/benpate/hannibal/tree/main/sigs)

This library is a simple-yet-thorough implementation of the IETF HTTP Signatures specification. It aims to be extensively tested and documented, with extensions for you to test and troubleshoot your own implementations.

## Project Status

This library is used in production as part of [Hannibal](https://github.com/benpate/hannibal) and [Emissary](https://emissary.dev), but it is still under active development and its API may change. If you'd like to use it in your own project, please [reach out](https://mastodon.social/@benpate) — I'm happy to help, and feedback from other developers is very welcome.

## Key Features

* Simple API with sensible defaults
* Complete implementation of all commonly used cipher and digest algorithms
* Extensive use of [zerolog](https://github.com/rs/zerolog) logging to simplify troubleshooting

## Signing Outbound Requests

The `sigs` library makes it easy to sign an outbound http.Request. It includes sensible defaults (shown below) so that most uses should "just work" with minimal configuration.

```go
// Generate a private key to sign. Real code would likely
// retrieve this from a database.
privateKey, err := rsa.GenerateKey(rand.Reader, 2048)

// Generate an http.Request to sign. Real code would likely
// set additional values in the outbound request.
request, _ := http.NewRequest("POST", "https://example.com", nil)

// Sign the Request with the public key ID and the Private Key. Yes, that's it.
if err := sigs.Sign(request, "https://example.com/@me#main-key", privateKey); err != nil {
	// handle error...
}

```

### Signing Options

In the event that you need to customize the way you sign a Request, you can pass one or more optional functions into the `Sign` function.

| Option | Description | Default |
|--------|-------------|---------|
| `SignerFields(...)` | Sets the field(s) to include when creating the "Signature" header. | `(request-target) host date digest` |
| `SignerSignatureHash(...)` | Sets the algorithm used to hash the "Signature" header. | `crypto.SHA256` |
| `SignerBodyDigest(...)` | Sets the algorithm used to create the "Digest" header. | `crypto.SHA256` |
| `SignerCreated(...)` / `SignerExpires(...)` | Set the `(created)` / `(expires)` signature timestamps. | — |
| `SignerMaxBodySize(bytes)` | Sets the largest body that the `Digest` is computed over. A larger body cannot be signed. | 1 MiB |

```go
// How to sign a request using additional options
err := sigs.Sign(
	request,
	"https://example.com/@me#main-key",
	privateKey,
	sigs.SignerFields("(request-target)", "(created)", "(expires)"),
	sigs.SignerBodyDigest(crypto.SHA512),
)
```

### Object Notation

In most cases, the above syntax is the simplest way to use `sigs`. However, the library also publishes the underlying objects used to sign http.Requests, which you can also access directly. For instance, you may need to do this if you need to use complex logic to determine what options to set.

```go
signer := sigs.NewSigner("https://example.com/@me#main-key", privateKey)
signer.With(sigs.SignerFields("content-type", "date"))

if err := signer.Sign(request); err != nil {
	// handle error...
}

```

## Verifying Inbound Requests

`Verify` takes a `PublicKeyFinder` — a `func(keyID string) (string, error)` that returns the PEM-encoded public key for a given key ID. This lets you look the key up however you like (from the remote actor's profile, a cache, a database). `Verify` returns the parsed `Signature` along with any error.

```go
// Define an http.Request. Real code would receive this request
// from a remote server in an http.Handler function.
request, _ := http.NewRequest("POST", "https://example.com", nil)

// A PublicKeyFinder returns the PEM-encoded public key for a key ID.
// Real code would fetch this from the remote user's profile.
keyFinder := func(keyID string) (string, error) {
	return lookupPublicKeyPEM(keyID)
}

// Verify the request has a valid signature from that key.
if _, err := sigs.Verify(request, keyFinder); err != nil {
	// handle error...
}
```

### Verification Options

In the event that you need to customize the way you verify a Request, you can pass one or more optional functions into the `Verify` function.

| Option | Description | Default |
|--------|-------------|---------|
| `VerifierFields(...)` | Sets the list of fields that MUST ALL be present in the signature. Additional fields are allowed in the signature, and will still be verified. | `(request-target) host date digest` |
| `VerifierBodyDigests(...)` | Sets the list of algorithms to accept from remote servers when they create a "Digest" header. ALL recognized digests must be valid to pass, and AT LEAST ONE of the algorithms must be from this list. | `crypto.SHA256` |
| `VerifierSignatureHashes(...)` | Sets the hashing algorithms to try when validating the "Signature" header. Validation fails if checks on ALL algorithms are unsuccessful. | `crypto.SHA256`, `crypto.SHA512` |
| `VerifierTimeout(...)` / `VerifierIgnoreTimeout()` | Tune or disable the signature freshness window. | — |
| `VerifierIgnoreBodyDigest()` | Skip body-digest verification. | — |
| `VerifierMaxBodySize(...)` | Sets the largest body, in bytes, that the Digest check reads. | 1 MiB |
| `WithHostname(hostname)` | Names the host that requests are signed for, in place of the request's own `Host`. A server behind a proxy that rewrites `Host` passes its public name. The verifier never reads `X-Forwarded-Host`. | the request's `Host` |

A verifier refuses a request whose `Date` is older or newer than the timeout allows, a request with no `Date` when the date must be signed, and a request that has a body and no `Digest` header.

```go
// How to verify a request using additional options
_, err := sigs.Verify(
	request,
	keyFinder,
	sigs.VerifierFields("(request-target)", "(created)", "(expires)"),
	sigs.VerifierSignatureHashes(crypto.SHA512),
)
```

### Object Notation

In most cases, the above syntax is the simplest way to use `sigs`. However, the library also publishes the underlying objects used to verify http.Requests, which you can also access directly. For instance, you may need to do this if you need to use complex logic to determine what options to set.

```go
verifier := sigs.NewVerifier()
verifier.Use(sigs.VerifierFields("content-type", "date"))

if _, err := verifier.Verify(request, keyFinder); err != nil {
	// handle error...
}

```

## Troubleshooting

The `sigs` library generates fine-grained debugging information with the zerolog structured logging library. By default, it sets the logging level to `Disabled` so that no logging information is written. If you need to see deeper into `sigs`, add the following into your application code:

```go
func main() {

	// zerolog.SetGlobalLevel(zerolog.Trace) // for a step-by-step trace of every sigs action.
	// zerolog.SetGlobalLevel(zerolog.Debug) // for higher-level debugging of signatures and verification
	
	// your code here...
}

```


## Supported Algorithms

### Signatures

* hs2019
* rsa-sha256
* rsa-sha512
* hmac-sha256 (In Progress)
* ecdsa-sha256 (In Progress)

### Digests

* sha256
* sha512

## References

* IETF Standard: https://datatracker.ietf.org/doc/html/draft-ietf-httpbis-message-signatures
* Mastodon Security Documentation: https://docs.joinmastodon.org/spec/security/