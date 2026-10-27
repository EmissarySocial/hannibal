package clients

import (
	"errors"
	"testing"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInnerClient is a streams.Client whose Load returns a scripted document (or
// error) and records the arguments it was called with. It also records calls to
// the delegate methods.
type mockInnerClient struct {
	loadResult    streams.Document
	loadErr       error
	lastLoadURL   string
	lastLoadOpts  []any
	savedDocument streams.Document
	deletedID     string
	rootClientSet bool
}

// Load records the URL and options it was called with, then returns the scripted document and error
func (c *mockInnerClient) Load(url string, options ...any) (streams.Document, error) {
	c.lastLoadURL = url
	c.lastLoadOpts = options
	return c.loadResult, c.loadErr
}

// Save records the document it was given
func (c *mockInnerClient) Save(document streams.Document) error {
	c.savedDocument = document
	return nil
}

// Delete records the document ID it was given
func (c *mockInnerClient) Delete(documentID string) error {
	c.deletedID = documentID
	return nil
}

// SetRootClient records that it was called, ignoring the client it was given
func (c *mockInnerClient) SetRootClient(streams.Client) {
	c.rootClientSet = true
}

// TestHashLookup_NoHash confirms a URL without a fragment is passed straight
// through to the inner client, with options spread intact.
func TestHashLookup_NoHash(t *testing.T) {

	inner := &mockInnerClient{
		loadResult: streams.NewDocument(map[string]any{vocab.PropertyID: "https://example.com/actor"}),
	}
	client := NewHashLookup(inner)

	result, err := client.Load("https://example.com/actor", "opt1", "opt2")
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/actor", inner.lastLoadURL)
	assert.Equal(t, "https://example.com/actor", result.ID())

	// Options must pass through individually (spread), not nested.
	assert.Equal(t, []any{"opt1", "opt2"}, inner.lastLoadOpts)
}

// TestHashLookup_OptionsSpread is the regression guard for the variadic-spread
// bug: on a HASH-url lookup, the base-document Load must receive each option
// individually, not a single nested slice.
func TestHashLookup_OptionsSpread(t *testing.T) {

	inner := &mockInnerClient{
		loadResult: streams.NewDocument(map[string]any{
			vocab.PropertyPublicKey: map[string]any{
				vocab.PropertyID: "https://example.com/actor#main-key",
			},
		}),
	}
	client := NewHashLookup(inner)

	_, _ = client.Load("https://example.com/actor#main-key", "opt1", "opt2")

	// The inner client must be loaded with the BASE url and the spread options.
	assert.Equal(t, "https://example.com/actor", inner.lastLoadURL)
	assert.Equal(t, []any{"opt1", "opt2"}, inner.lastLoadOpts,
		"options must be spread, not wrapped in a nested slice")
}

// TestHashLookup_FindInProperty confirms the fragment is resolved to a top-level
// property whose ID matches the full URL.
func TestHashLookup_FindInProperty(t *testing.T) {

	fullURL := "https://example.com/actor#main-key"
	inner := &mockInnerClient{
		loadResult: streams.NewDocument(map[string]any{
			vocab.PropertyID: "https://example.com/actor",
			vocab.PropertyPublicKey: map[string]any{
				vocab.PropertyID:           fullURL,
				vocab.PropertyPublicKeyPEM: "PEM-DATA",
			},
		}),
	}
	client := NewHashLookup(inner)

	result, err := client.Load(fullURL)
	require.NoError(t, err)
	assert.Equal(t, fullURL, result.ID())
	assert.Equal(t, "PEM-DATA", result.PublicKeyPEM())
}

// TestHashLookup_FindInArray confirms the fragment is resolved when the matching
// object lives inside an array-valued property.
func TestHashLookup_FindInArray(t *testing.T) {

	fullURL := "https://example.com/actor#key-2"
	inner := &mockInnerClient{
		loadResult: streams.NewDocument(map[string]any{
			vocab.PropertyID: "https://example.com/actor",
			vocab.PropertyPublicKey: []any{
				map[string]any{vocab.PropertyID: "https://example.com/actor#key-1"},
				map[string]any{vocab.PropertyID: fullURL, vocab.PropertyPublicKeyPEM: "SECOND"},
			},
		}),
	}
	client := NewHashLookup(inner)

	result, err := client.Load(fullURL)
	require.NoError(t, err)
	assert.Equal(t, fullURL, result.ID())
	assert.Equal(t, "SECOND", result.PublicKeyPEM())
}

// TestHashLookup_NotFound confirms a fragment with no matching sub-object returns
// a NotFound error.
func TestHashLookup_NotFound(t *testing.T) {

	inner := &mockInnerClient{
		loadResult: streams.NewDocument(map[string]any{
			vocab.PropertyID:   "https://example.com/actor",
			vocab.PropertyName: "Alice",
		}),
	}
	client := NewHashLookup(inner)

	result, err := client.Load("https://example.com/actor#missing-key")
	require.Error(t, err)
	assert.True(t, derp.IsNotFound(err))
	assert.True(t, result.IsNil())
}

// TestHashLookup_InnerError confirms an error loading the base document is
// propagated.
func TestHashLookup_InnerError(t *testing.T) {

	innerErr := errors.New("network failure")
	inner := &mockInnerClient{
		loadErr: innerErr,
	}
	client := NewHashLookup(inner)

	_, err := client.Load("https://example.com/actor#main-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, innerErr)
}

// TestHashLookup_Delegates confirms Save, Delete, and SetRootClient pass through
// to the inner client (so HashLookup is a transparent streams.Client wrapper).
func TestHashLookup_Delegates(t *testing.T) {

	inner := &mockInnerClient{}
	client := NewHashLookup(inner)

	document := streams.NewDocument(map[string]any{vocab.PropertyID: "https://example.com/1"})
	require.NoError(t, client.Save(document))
	assert.Equal(t, "https://example.com/1", inner.savedDocument.ID())

	require.NoError(t, client.Delete("https://example.com/2"))
	assert.Equal(t, "https://example.com/2", inner.deletedID)

	client.SetRootClient(client)
	assert.True(t, inner.rootClientSet)
}
