package streams

import (
	"slices"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/remote"
)

// DefaultClient is a default implementation of the hannibal.Client interface.
// It simply loads ActivityStream documents from remote servers with no caching
type DefaultClient struct {
	options []remote.Option
}

// NewDefaultClient returns a DefaultClient that loads documents from remote servers using the provided options.
func NewDefaultClient(options ...remote.Option) Client {
	return DefaultClient{
		options: options,
	}
}

// Load implements the hannibal.Client interface, which loads an ActivityStream
// document from a remote server. For the hannibal default client, this method
// simply loads the document from a remote server with no other processing.
func (client DefaultClient) Load(url string, options ...any) (Document, error) {

	const location = "hannibal.streams.Client.Load"

	result := make(map[string]any)

	// Collect options from both the client and the method call. Concat always allocates;
	// append would write per-call options into the backing array this client shares.
	remoteOptions := slices.Concat(client.options, remote.Options(options...))

	// Try to load-and-parse the value from the remote server. Options run when the request is
	// sent, after MaxResponseSize, so a caller can still raise the cap for a larger document.
	transaction := remote.Get(url).
		Accept(vocab.ContentTypeActivityPub).
		MaxResponseSize(MaxResponseSize).
		With(remoteOptions...).
		Result(&result)

	if err := transaction.Send(); err != nil {
		return NilDocument(), derp.Wrap(err, location, "Unable to load JSON-LD document", url)
	}

	// Return in triumph
	return NewDocument(result,
			WithClient(client),
			WithHTTPHeader(transaction.ResponseHeader()),
		),
		nil
}

// Save is required to implement the document.Cache interface.
// For this client, Save is a NOOP
func (client DefaultClient) Save(_ Document) error {
	return nil
}

// Delete is required to implement the document.Cache interface.
// For this client, Delete is a NOOP
func (client DefaultClient) Delete(_ string) error {
	return nil
}

// SetRootClient is required to implement the hannibal.Client interface.
// For this client, SetRootClient is a NOOP
func (client DefaultClient) SetRootClient(_ Client) {}
