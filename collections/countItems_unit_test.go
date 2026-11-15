package collections

import (
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCountItems_TotalItems confirms the fast path: when the collection reports
// a positive TotalItems, that value is returned without paging.
func TestCountItems_TotalItems(t *testing.T) {

	collection := streams.NewDocument(map[string]any{
		vocab.PropertyType:       vocab.CoreTypeOrderedCollection,
		vocab.PropertyTotalItems: 42,
	})

	count, err := CountItems(collection)
	require.NoError(t, err)
	assert.Equal(t, 42, count)
}

// TestCountItems_Nil confirms a Nil collection counts as zero items.
func TestCountItems_Nil(t *testing.T) {

	count, err := CountItems(streams.NilDocument())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestCountItems_ByPaging confirms that when no TotalItems is reported, the items
// are counted by walking the (single, inline) page.
func TestCountItems_ByPaging(t *testing.T) {

	collection := inlineCollection(
		"https://example.com/1",
		"https://example.com/2",
		"https://example.com/3",
	)

	count, err := CountItems(collection)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

// TestCountItems_EmptyPage confirms a collection with no items and no TotalItems
// counts as zero.
func TestCountItems_EmptyPage(t *testing.T) {

	collection := streams.NewDocument(map[string]any{
		vocab.PropertyType:  vocab.CoreTypeOrderedCollection,
		vocab.PropertyItems: []any{},
	})

	count, err := CountItems(collection)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestCountItems_Options confirms the caller's caps bound how much of a collection is counted
func TestCountItems_Options(t *testing.T) {

	// The two pages point at each other forever, one item each
	count, err := CountItems(cyclicCollection(), WithMaxPages(5))
	require.NoError(t, err)
	assert.Equal(t, 5, count)

	count, err = CountItems(cyclicCollection(), WithMaxPages(2))
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

// panicClient is a streams.Client that panics whenever it is asked to load anything
type panicClient struct{}

// SetRootClient is a NOOP for this stub
func (panicClient) SetRootClient(streams.Client) {}

// Load always panics
func (panicClient) Load(string, ...any) (streams.Document, error) {
	panic("the client failed")
}

// Save is a NOOP for this stub
func (panicClient) Save(streams.Document) error { return nil }

// Delete is a NOOP for this stub
func (panicClient) Delete(string) error { return nil }

// TestCountItems_PanicIsAnError confirms a panic is reported as an error, and never as an
// empty collection
func TestCountItems_PanicIsAnError(t *testing.T) {

	collection := streams.NewDocument(map[string]any{
		vocab.PropertyID:    "https://example.com/replies",
		vocab.PropertyType:  vocab.CoreTypeCollection,
		vocab.PropertyFirst: "https://example.com/replies?page=1",
	}, streams.WithClient(panicClient{}))

	count, err := CountItems(collection)
	require.Error(t, err)
	assert.Zero(t, count)
}
