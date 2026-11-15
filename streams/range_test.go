package streams

import (
	"slices"
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocument_Range confirms Range yields each element of a slice document as
// its own sub-document, in order.
func TestDocument_Range(t *testing.T) {

	doc := NewDocument([]any{"a", "b", "c"})

	var collected []string
	for item := range doc.Range() {
		collected = append(collected, item.String())
	}

	assert.Equal(t, []string{"a", "b", "c"}, collected)
}

// TestDocument_Range_Scalar confirms a scalar document yields exactly one item.
func TestDocument_Range_Scalar(t *testing.T) {

	doc := NewDocument("only")

	var collected []string
	for item := range doc.Range() {
		collected = append(collected, item.String())
	}

	assert.Equal(t, []string{"only"}, collected)
}

// TestDocument_Range_EarlyStop confirms Range honors an early break.
func TestDocument_Range_EarlyStop(t *testing.T) {

	doc := NewDocument([]any{"a", "b", "c", "d"})

	var collected []string
	for item := range doc.Range() {
		collected = append(collected, item.String())
		if item.String() == "b" {
			break
		}
	}

	assert.Equal(t, []string{"a", "b"}, collected)
}

// TestDocument_RangeWithIndex confirms RangeWithIndex yields ascending indexes.
func TestDocument_RangeWithIndex(t *testing.T) {

	doc := NewDocument([]any{"a", "b", "c"})

	indexes := make([]int, 0)
	values := make([]string, 0)
	for index, item := range doc.RangeWithIndex() {
		indexes = append(indexes, index)
		values = append(values, item.String())
	}

	assert.Equal(t, []int{0, 1, 2}, indexes)
	assert.Equal(t, []string{"a", "b", "c"}, values)
}

// TestDocument_RangeIDs confirms RangeIDs yields the id of each element.
func TestDocument_RangeIDs(t *testing.T) {

	doc := NewDocument([]any{
		map[string]any{vocab.PropertyID: "https://example.com/1"},
		map[string]any{vocab.PropertyID: "https://example.com/2"},
	})

	var ids []string
	for id := range doc.RangeIDs() {
		ids = append(ids, id)
	}

	assert.Equal(t, []string{"https://example.com/1", "https://example.com/2"}, ids)
}

// TestDocument_RangeMentions confirms RangeMentions yields the href of each tag
// whose type is Mention, skipping other tag types.
func TestDocument_RangeMentions(t *testing.T) {

	doc := NewDocument(map[string]any{
		vocab.PropertyTag: []any{
			map[string]any{
				vocab.PropertyType: vocab.LinkTypeMention,
				vocab.PropertyHref: "https://example.com/@alice",
			},
			map[string]any{
				vocab.PropertyType: "Hashtag",
				vocab.PropertyHref: "https://example.com/tags/go",
			},
			map[string]any{
				vocab.PropertyType: vocab.LinkTypeMention,
				vocab.PropertyHref: "https://example.com/@bob",
			},
		},
	})

	var mentions []string
	for href := range doc.RangeMentions() {
		mentions = append(mentions, href)
	}

	assert.Equal(t, []string{"https://example.com/@alice", "https://example.com/@bob"}, mentions)
}

// TestDocument_IDs verifies each shape that a property can take, that an element with no ID
// is skipped, and that IDs and RangeIDs agree
func TestDocument_IDs(t *testing.T) {

	const alice = "https://example.com/users/alice"
	const bob = "https://example.com/users/bob"

	type namedList []any
	type namedMap map[string]any

	for name, test := range map[string]struct {
		value    any
		expected sliceof.String
	}{
		"nil":               {nil, sliceof.String{}},
		"string":            {alice, sliceof.String{alice}},
		"empty string":      {"", sliceof.String{}},
		"object":            {map[string]any{"id": alice}, sliceof.String{alice}},
		"JSON-LD id":        {map[string]any{"@id": alice}, sliceof.String{alice}},
		"object without id": {map[string]any{"type": "Person"}, sliceof.String{}},
		"number":            {42, sliceof.String{}},
		"list":              {[]any{alice, map[string]any{"id": bob}, 42, map[string]any{}, ""}, sliceof.String{alice, bob}},
		"list of nothing":   {[]any{map[string]any{}, 42}, sliceof.String{}},
		"list of strings":   {[]string{alice, bob}, sliceof.String{alice, bob}},
		"sliceof.String":    {sliceof.String{alice}, sliceof.String{alice}},
		"mapof.Any":         {mapof.Any{"id": alice}, sliceof.String{alice}},
		"named list":        {namedList{alice, namedMap{"id": bob}}, sliceof.String{alice, bob}},
		"named map":         {namedMap{"id": alice}, sliceof.String{alice}},
		"email":             {"mailto:dana@example.com", sliceof.String{"mailto:dana@example.com"}},
	} {
		t.Run(name, func(t *testing.T) {

			property := NewDocument(map[string]any{"to": test.value}).Get("to")

			require.Equal(t, test.expected, property.IDs())
			require.Equal(t, []string(test.expected), slices.AppendSeq(make([]string, 0), property.RangeIDs()))
		})
	}
}

// TestDocument_IDs_LoadsNothing verifies that reading the IDs of bare URLs never asks the
// Document's client for them
func TestDocument_IDs_LoadsNothing(t *testing.T) {

	client := newRecordingClient()

	document := NewDocument(map[string]any{
		"to":    []any{"https://example.com/users/alice", "https://example.com/users/bob"},
		"actor": "https://example.com/users/carol",
	}, WithClient(client))

	require.Len(t, document.To().IDs(), 2)
	require.Equal(t, sliceof.String{"https://example.com/users/carol"}, document.Get("actor").IDs())
	require.Empty(t, client.options, "no URL was loaded")
}

// TestDocument_RangeIDs_StopsEarly verifies that the iterator stops when its consumer does
func TestDocument_RangeIDs_StopsEarly(t *testing.T) {

	document := NewDocument([]any{"https://example.com/1", map[string]any{}, "https://example.com/2", "https://example.com/3"})

	seen := make([]string, 0)

	for id := range document.RangeIDs() {
		seen = append(seen, id)

		if len(seen) == 2 {
			break
		}
	}

	require.Equal(t, []string{"https://example.com/1", "https://example.com/2"}, seen)
}

// TestDocument_Recipients_SkipsWhatHasNoID verifies that a recipient written as an object is
// read by its id, and that one with no id is left out
func TestDocument_Recipients_SkipsWhatHasNoID(t *testing.T) {

	document := NewDocument(map[string]any{
		"to":  []any{map[string]any{"id": "https://example.com/users/alice"}, map[string]any{"type": "Person"}},
		"cc":  "https://example.com/users/bob",
		"bto": []any{},
		"bcc": []any{42, "https://example.com/users/dave"},
	})

	require.Equal(t, sliceof.String{
		"https://example.com/users/alice",
		"https://example.com/users/bob",
		"https://example.com/users/dave",
	}, document.Recipients())
}
