package streams

import (
	"github.com/benpate/derp"
	"github.com/rs/zerolog/log"
)

// NilClient is the Client of a Document that was made without one. It loads nothing, so a
// Document built from local data can never reach the network on its own.
type NilClient struct{}

// NewNilClient returns a Client that refuses every load
func NewNilClient() Client {
	return NilClient{}
}

// Load implements the Client interface. It makes no request, and returns an error that names the URL.
func (client NilClient) Load(url string, _ ...any) (Document, error) {

	const location = "hannibal.streams.NilClient.Load"

	// A getter that loads a bare URL discards this error and reads the value as empty, so the
	// log line is the only trace of a Document that needed a client and had none
	log.Debug().Str("location", location).Str("url", url).Msg("Document has no client. Nothing was loaded.")

	return NilDocument(), derp.Internal(location, "Document has no client, so nothing can be loaded", url)
}

// Save is required to implement the Client interface.
// For this client, Save is a NOOP
func (client NilClient) Save(_ Document) error {
	return nil
}

// Delete is required to implement the Client interface.
// For this client, Delete is a NOOP
func (client NilClient) Delete(_ string) error {
	return nil
}

// SetRootClient is required to implement the Client interface.
// For this client, SetRootClient is a NOOP
func (client NilClient) SetRootClient(_ Client) {}
