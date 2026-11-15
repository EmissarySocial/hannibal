package sender

import (
	"crypto"

	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/remote"
)

// signRequest is a middleware for the remote package that adds an HTTP Signature to a request.
func signRequest(publicKeyID string, privateKey crypto.PrivateKey) remote.Option {
	return sigs.WithSigner(sigs.NewSigner(publicKeyID, privateKey))
}
