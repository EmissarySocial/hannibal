package streams

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestContextEntry_WithExtension_ZeroValue confirms WithExtension works on an entry built without
// NewContextEntry, whose Extensions map is nil
func TestContextEntry_WithExtension_ZeroValue(t *testing.T) {

	entry := ContextEntry{Vocabulary: "https://test.com"}
	require.Nil(t, entry.Extensions)

	result := entry.WithExtension("dog", "https://dog.com/ns/activitystreams")

	require.Same(t, &entry, result)
	require.Equal(t, map[string]string{"dog": "https://dog.com/ns/activitystreams"}, entry.Extensions)
}
