package streams

import (
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
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
