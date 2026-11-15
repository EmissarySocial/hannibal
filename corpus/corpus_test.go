package corpus

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testWorld is the World that these tests rewrite the fediverse examples for
var testWorld = World{
	Actor:     "https://example.com/@author",
	Followers: "https://example.com/@author/pub/followers",
	Members:   []string{"https://remote.example/@alice", "mailto:dana@example.org"},
}

// TestFediverse verifies that every fediverse example parses, names its source and its
// recipients, and has had its tokens replaced
func TestFediverse(t *testing.T) {

	examples, err := Fediverse(testWorld)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(examples), 70, "examples have gone missing")

	names := make(map[string]bool)

	for _, example := range examples {
		t.Run(example.Name, func(t *testing.T) {

			require.False(t, names[example.Name], "an example name is used twice")
			names[example.Name] = true

			require.NotEmpty(t, example.Source)
			require.NotNil(t, example.Recipients, "a fediverse example records its recipients, even when there are none")
			require.NotContains(t, string(example.JSON), "{{", "a token was not replaced")

			for _, recipient := range example.Recipients {
				require.NotContains(t, recipient, "{{", "a token was not replaced")
			}

			document, err := example.Document()
			require.NoError(t, err)
			require.NotEmpty(t, document)
		})
	}
}

// TestFediverse_RewritesTheWorld verifies that an example carries the World's actor and
// collection, and that a recorded collection is replaced by its members
func TestFediverse_RewritesTheWorld(t *testing.T) {

	examples, err := Fediverse(testWorld)
	require.NoError(t, err)

	for _, example := range examples {

		if example.Name != "mastodon-create-note-public" {
			continue
		}

		document, err := example.Document()
		require.NoError(t, err)
		require.Equal(t, testWorld.Actor, document["actor"])
		require.Equal(t, []any{testWorld.Followers}, document["cc"])
		require.Equal(t, testWorld.Members, example.Recipients)
		return
	}

	t.Fatal("the example was not found")
}

// TestVerbatim verifies that every W3C document parses as a JSON object, and records no recipients
func TestVerbatim(t *testing.T) {

	published, err := W3C()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(published), 200, "examples have gone missing")

	rejected, err := Rejected()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rejected), 16, "examples have gone missing")

	for _, example := range append(published, rejected...) {
		t.Run(example.Name, func(t *testing.T) {

			require.NotEmpty(t, example.Source)
			require.Nil(t, example.Recipients)
			require.False(t, strings.HasSuffix(example.Name, ".json"))

			_, err := example.Document()
			require.NoError(t, err)
		})
	}
}

// TestExample_Document verifies that each call returns its own value, and that a document
// that is not a JSON object is an error
func TestExample_Document(t *testing.T) {

	example := Example{Name: "test", JSON: []byte(`{"to":["https://example.com/@bob"]}`)}

	first, err := example.Document()
	require.NoError(t, err)

	first["to"].([]any)[0] = "changed"

	second, err := example.Document()
	require.NoError(t, err)
	require.Equal(t, []any{"https://example.com/@bob"}, second["to"])

	_, err = Example{Name: "bad", JSON: []byte(`[1,2,3]`)}.Document()
	require.Error(t, err)
}

// TestExpandRecipients verifies that tokens take their real values, a collection becomes its
// members, and an address keeps the place where it first appears
func TestExpandRecipients(t *testing.T) {

	world := World{
		Actor:     "https://example.com/@author",
		Followers: "https://example.com/@author/pub/followers",
		Members:   []string{"https://a.example/1", "https://b.example/2"},
	}

	require.Equal(t,
		[]string{"https://b.example/2", "https://a.example/1", "https://example.com/@author", "https://c.example/3"},
		expandRecipients([]string{"https://b.example/2", tokenFollowers, tokenActor, "https://c.example/3", tokenFollowers, "https://a.example/1"}, world),
	)

	require.Empty(t, expandRecipients(nil, world))
	require.NotNil(t, expandRecipients(nil, world))
	require.Empty(t, expandRecipients([]string{tokenFollowers}, World{}))
}

// TestExampleNames_MissingFolder verifies that a folder that does not exist is an error
func TestExampleNames_MissingFolder(t *testing.T) {

	_, err := exampleNames("missing")
	require.Error(t, err)

	_, err = verbatim("missing", "nothing")
	require.Error(t, err)
}
