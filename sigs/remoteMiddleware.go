package sigs

import (
	"net/http"

	"github.com/benpate/derp"
	"github.com/benpate/remote"
)

// WithSigner is a remote.Option that signs an outbound HTTP request, and refuses to send a
// request that its key cannot sign
func WithSigner(signer Signer) remote.Option {

	const location = "hannibal.sigs.WithSigner"

	return remote.Option{

		ModifyRequest: func(txn *remote.Transaction, request *http.Request) (*http.Response, error) {

			// RULE: A request that cannot be signed is never sent. The refusal reads as a client
			// error so that a caller does not retry it: a key that cannot sign now will not sign later.
			if err := signer.Sign(request); err != nil {
				return nil, derp.Wrap(err, location, "Request refused, because it cannot be signed", "keyID: "+signer.PublicKeyID, derp.WithBadRequest())
			}

			// If exists, write the Digest back into the transaction (for serialization, et al)
			if digest := request.Header.Get("Digest"); digest != "" {
				txn.Header("Digest", digest)
			}

			// If exists, write the Signature back into the transaction (for serialization, et al)
			if signature := request.Header.Get("Signature"); signature != "" {
				txn.Header("Signature", signature)
			}

			// Nil response means that we are still sending the request to the remote server
			// instead of replacing it with a new request.
			return nil, nil
		},
	}
}
