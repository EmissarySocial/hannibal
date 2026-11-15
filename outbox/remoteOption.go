package outbox

import (
	"github.com/benpate/hannibal/sigs"
	"github.com/benpate/remote"
)

// SignRequest is a middleware for the remote package that adds an HTTP Signature to a request.
func SignRequest(actor Actor) remote.Option {
	return sigs.WithSigner(sigs.NewSigner(actor.publicKeyID, actor.privateKey))
}
