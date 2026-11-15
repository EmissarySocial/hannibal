package streams

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestImage_NeverReturnsRawRemoteStrings verifies that an Image's text has no HTML, and that
// its address is a web address or nothing
func TestImage_NeverReturnsRawRemoteStrings(t *testing.T) {

	for name, href := range map[string]string{
		"script":          "javascript:alert(1)",
		"data":            "data:text/html,<script>alert(1)</script>",
		"relative":        "/local/file.png",
		"scheme relative": "//evil.example/x.png",
		"no host":         "https:///x.png",
		"file":            "file:///etc/passwd",
		"empty":           "",
	} {
		require.Empty(t, NewImage(href).Href(), name)
		require.Empty(t, NewImage(map[string]any{"href": href}).Href(), name)
		require.Empty(t, NewImage(map[string]any{"url": href}).URL(), name)
		require.Empty(t, NewImage([]any{map[string]any{"url": href}}).URL(), name)
	}

	require.Equal(t, "https://example.com/a.png?x=1&y=2", NewImage("https://example.com/a.png?x=1&y=2").Href())
	require.Equal(t, "http://example.com/a.png", NewImage([]any{map[string]any{"url": "http://example.com/a.png"}}).URL())

	// The actor document from the defect report
	actor := NewDocument(map[string]any{"icon": map[string]any{"url": "javascript:alert(1)", "summary": "<b>My</b> <script>alert(1)</script>avatar &amp; me", "mediaType": "image/png<script>"}})

	require.Empty(t, actor.Icon().URL())
	require.Equal(t, "My avatar & me", actor.Icon().Summary())
	require.Equal(t, "image/png", actor.Icon().MediaType())
	require.Equal(t, "image/png", NewImage([]any{map[string]any{"mediaType": "image/png"}}).MediaType())
}
