package streams

import (
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/require"
)

// testClient is an in-memory Client that serves documents from a fixed map
// nolint:unused
type testClient struct {
	data mapof.Any
}

// SetRootClient is a NOOP for this stub
// nolint:unused
func (client testClient) SetRootClient(_ Client) {}

// Load returns the document stored under this URI, or an error when there is none
// nolint:unused
func (client testClient) Load(uri string, _ ...any) (Document, error) {

	if value, ok := client.data[uri]; ok {
		return NewDocument(value, WithClient(client)), nil
	}

	return NilDocument(), derp.Internal("hannibal.streams.testClient.Load", "Unknown URI", uri)
}

// Save is a NOOP for this stub
func (client testClient) Save(_ Document) error {
	return nil
}

// Delete is a NOOP for this stub
func (client testClient) Delete(_ string) error {
	return nil
}

// TestTestClient confirms the testClient stub's Save and Delete succeed
func TestTestClient(t *testing.T) {

	// this is just a hack to make the "unused" linting messages go away
	client := testClient{}

	document := NewDocument(nil, WithClient(client))

	require.Nil(t, client.Save(document))
	require.Nil(t, client.Delete(document.ID()))
}
