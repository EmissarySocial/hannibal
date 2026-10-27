package validator

import (
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient is a streams.Client that serves documents from an in-memory map,
// so validator tests can resolve object/actor references without a network.
type mockClient struct {
	data map[string]map[string]any
}

func (client mockClient) SetRootClient(streams.Client) {}

func (client mockClient) Load(uri string, _ ...any) (streams.Document, error) {
	if value, ok := client.data[uri]; ok {
		return streams.NewDocument(value, streams.WithClient(client)), nil
	}
	return streams.NilDocument(), derp.NotFound("validator.mockClient.Load", "Unknown URI", uri)
}

func (client mockClient) Save(_ streams.Document) error { return nil }
func (client mockClient) Delete(_ string) error         { return nil }

// TestHTTPLookup_NonCreateUpdate confirms the validator abstains (Unknown) for
// activity types other than Create and Update.
func TestHTTPLookup_NonCreateUpdate(t *testing.T) {

	v := NewHTTPLookup()

	activity := streams.NewDocument(map[string]any{
		vocab.PropertyType: vocab.ActivityTypeLike,
	})

	assert.Equal(t, ResultUnknown, v.Validate(blankRequest(), &activity))
}

// TestHTTPLookup_FetchesOriginal confirms that, for a Create activity, the
// validator fetches the original object from its origin server and replaces the
// activity's value with the retrieved copy.
func TestHTTPLookup_FetchesOriginal(t *testing.T) {

	objectID := "https://example.com/notes/1"

	// The origin server's canonical copy carries content not present in the
	// inbound activity, so we can confirm the value was actually replaced.
	client := mockClient{data: map[string]map[string]any{
		objectID: {
			vocab.PropertyID:      objectID,
			vocab.PropertyType:    vocab.ObjectTypeNote,
			vocab.PropertyContent: "canonical content",
		},
	}}

	v := NewHTTPLookup()
	activity := streams.NewDocument(map[string]any{
		vocab.PropertyType:   vocab.ActivityTypeCreate,
		vocab.PropertyObject: objectID,
	}, streams.WithClient(client))

	require.Equal(t, ResultValid, v.Validate(blankRequest(), &activity))
	assert.Equal(t, "canonical content", activity.Content())
}
