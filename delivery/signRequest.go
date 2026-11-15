package delivery

import (
	"crypto"

	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/remote"
)

// signRequest returns a remote option that signs each outgoing request with the actor's key.
// It signs inside the request, so that every attempt carries a fresh date and signature.
func signRequest(publicKeyID string, privateKey crypto.PrivateKey) remote.Option {
	return sigs.WithSigner(sigs.NewSigner(publicKeyID, privateKey))
}
