package streams

import (
	"html"

	"github.com/microcosm-cc/bluemonday"
)

// strictPolicy is the shared policy that strips all HTML. Callers must never modify it.
var strictPolicy = bluemonday.StrictPolicy()

// ugcPolicy is the shared policy that allows user-generated HTML. Callers must never modify it.
var ugcPolicy = bluemonday.UGCPolicy()

// plainText returns a string from another server with all of its HTML removed and its entities
// decoded, which is what Document.String returns
func plainText(value string) string {
	return html.UnescapeString(strictPolicy.Sanitize(value))
}
