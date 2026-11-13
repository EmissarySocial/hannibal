package delivery

import (
	"crypto"
	"net/http"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/remote"
)

// signRequest returns a remote option that signs each outgoing request with the actor's key.
// It signs inside the request, so that every attempt carries a fresh date and signature.
func signRequest(publicKeyID string, privateKey crypto.PrivateKey) remote.Option {

	const location = "hannibal.delivery.signRequest"

	return remote.Option{
		ModifyRequest: func(transaction *remote.Transaction, request *http.Request) *http.Response {

			// Add the Digest header and the Signature. The signed fields include the host, so a
			// signature made for one server is no use at any other.
			if err := sigs.Sign(request, publicKeyID, privateKey); err != nil {
				derp.Report(derp.Wrap(err, location, "Unable to sign HTTP request. This is likely a problem with the actor's private key."))
			}

			// Copy both headers back onto the transaction, so that its record of the request is complete
			if digest := request.Header.Get("Digest"); digest != "" {
				transaction.Header("Digest", digest)
			}

			if signature := request.Header.Get("Signature"); signature != "" {
				transaction.Header("Signature", signature)
			}

			return nil
		},
	}
}
