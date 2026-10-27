package streams

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNew confirms that DefaultContext and NewContext build the expected entries, and that
// languages, extensions, and multiple entries marshal to the expected JSON-LD @context.
func TestNew(t *testing.T) {

	// Test default context data
	{
		c := DefaultContext()

		require.True(t, c.IsEmptyTail())
		require.True(t, c.Tail().IsEmpty())
		head := c.Head()

		if head == nil {
			t.Fatal("Head() returned nil")
			return
		}

		require.Equal(t, "https://www.w3.org/ns/activitystreams", head.Vocabulary)
		require.Equal(t, "und", head.Language)
		require.Zero(t, len(head.Extensions))

		result, err := c.MarshalJSON()
		require.Nil(t, err)
		require.Equal(t, `"https://www.w3.org/ns/activitystreams"`, string(result))
	}

	// Test custom context, and chaining multiple contexts
	{
		c := NewContext()
		entry := c.Add("https://test.com").WithLanguage("en-us")
		head := c.Head()

		if head == nil {
			t.Fatal("Head() returned nil")
			return
		}

		// Verify the head entry
		require.Equal(t, "https://test.com", head.Vocabulary)
		require.Equal(t, "en-us", head.Language)
		require.Zero(t, len(head.Extensions))

		{
			result, err := json.Marshal(c)
			require.Nil(t, err)
			require.Equal(t, `{"@language":"en-us","@vocab":"https://test.com"}`, string(result))
		}

		entry.WithExtension("ext", "https://extension.com/ns/activitystreams")

		json1, err1 := c.MarshalJSON()
		require.Nil(t, err1)
		require.Equal(t, `{"@language":"en-us","@vocab":"https://test.com","ext":"https://extension.com/ns/activitystreams"}`, string(json1))

		c.Add("https://www.w3.org/ns/activitystreams")
		json2, err2 := c.MarshalJSON()

		require.Equal(t, `[{"@language":"en-us","@vocab":"https://test.com","ext":"https://extension.com/ns/activitystreams"},"https://www.w3.org/ns/activitystreams"]`, string(json2))
		require.Nil(t, err2)
	}

	// Test adding an extension to an entry whose language was never set
	{
		c := NewContext()
		c.Add("https://test.com").
			WithExtension("dog", "https://dog.com/ns/activitystreams")

		head := c.Head()

		if head == nil {
			t.Fatal("Head() returned nil")
			return
		}

		require.Equal(t, "https://test.com", head.Vocabulary)
		require.Equal(t, "und", head.Language)
		require.Equal(t, head.Extensions["dog"], "https://dog.com/ns/activitystreams")
	}
}
