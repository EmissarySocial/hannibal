package sigs

import "crypto"

// SignerOption is a function that modifies a Signer
type SignerOption func(*Signer)

// SignerFields sets the http.Request fields to be signed
func SignerFields(fields ...string) SignerOption {
	return func(signer *Signer) {
		signer.Fields = fields
	}
}

// SignerSignatureHash sets the hashing algorithm used when signing a request.
func SignerSignatureHash(hash crypto.Hash) SignerOption {
	return func(signer *Signer) {
		signer.SignatureHash = hash
	}
}

// SignerBodyDigest sets the digest algorithm used when creating the "Digest" header.
func SignerBodyDigest(digest crypto.Hash) SignerOption {
	return func(signer *Signer) {
		signer.BodyDigest = digest
	}
}

// SignerCreated sets the "created" timestamp (Unix epoch seconds) of the signature.
func SignerCreated(created int64) SignerOption {
	return func(signer *Signer) {
		signer.Created = created
	}
}

// SignerExpires sets the "expires" timestamp (Unix epoch seconds) of the signature.
func SignerExpires(expires int64) SignerOption {
	return func(signer *Signer) {
		signer.Expires = expires
	}
}

// SignerMaxBodySize sets the largest body, in bytes, that the Digest is computed over. Zero or
// less restores the default. A larger body cannot be signed.
func SignerMaxBodySize(maxBytes int64) SignerOption {
	return func(signer *Signer) {
		signer.MaxBodySize = maxBytes
	}
}
