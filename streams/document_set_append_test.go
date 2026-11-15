package streams

import (
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
)

// TestDocument_AppendAddressing confirms the remaining addressing append helpers
// (To/BTo/BCC) accumulate values and NOOP on an empty string. (CC is covered in
// document_set_extra_test.go.)
func TestDocument_AppendAddressing(t *testing.T) {

	check := func(name string, property string, appendFn func(*Document, string) bool) {
		t.Run(name, func(t *testing.T) {

			doc := NewDocument(map[string]any{})

			// Appending an empty value is a NOOP and reports false.
			assert.False(t, appendFn(&doc, ""))

			// Appending two values accumulates both, in order.
			assert.True(t, appendFn(&doc, "https://example.com/users/alice"))
			assert.True(t, appendFn(&doc, "https://example.com/users/bob"))

			got := doc.Get(property).SliceOfString()
			assert.Equal(t, []string{"https://example.com/users/alice", "https://example.com/users/bob"}, []string(got))
		})
	}

	check("AppendTo", vocab.PropertyTo, (*Document).AppendTo)
	check("AppendBTo", vocab.PropertyBTo, (*Document).AppendBTo)
	check("AppendBCC", vocab.PropertyBCC, (*Document).AppendBCC)
}
