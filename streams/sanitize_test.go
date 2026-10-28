package streams

import (
	"html"
	"sync"
	"testing"

	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/require"
)

// sanitizeCorpus holds inputs that exercise both policies' allow and deny rules.
var sanitizeCorpus = []string{
	"",
	"plain text",
	"https://example.com/users/alice?a=1&amp;b=2",
	"John <i>Connor</i>",
	"This is a <b>bad</b> summary <script>alert('hey there')</script>",
	"<p>Some <a href=\"https://example.com\" onclick=\"x()\">link</a> and <img src=x onerror=alert(1)></p>",
	"&lt;img src=x onerror=alert(1)&gt;",
	"<style>p{color:red}</style><iframe src=\"https://evil.example\"></iframe><table><tr><td>cell</td></tr></table>",
	"-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA\n-----END PUBLIC KEY-----",
}

// TestSanitize_MatchesFreshPolicies confirms the shared policies sanitize exactly as newly built ones do.
func TestSanitize_MatchesFreshPolicies(t *testing.T) {

	for _, input := range sanitizeCorpus {
		document := NewDocument(input)

		expectedString := html.UnescapeString(bluemonday.StrictPolicy().Sanitize(input))
		require.Equal(t, expectedString, document.String(), input)

		expectedHTML := bluemonday.UGCPolicy().Sanitize(input)
		require.Equal(t, expectedHTML, document.HTMLString(), input)
	}
}

// TestSanitize_Concurrent confirms the shared policies are safe to use from many goroutines at once.
func TestSanitize_Concurrent(t *testing.T) {

	// Compute the expected results before any goroutine touches the shared policies
	expectedStrings := make([]string, len(sanitizeCorpus))
	expectedHTML := make([]string, len(sanitizeCorpus))

	for index, input := range sanitizeCorpus {
		expectedStrings[index] = html.UnescapeString(bluemonday.StrictPolicy().Sanitize(input))
		expectedHTML[index] = bluemonday.UGCPolicy().Sanitize(input)
	}

	// Sanitize every input from many goroutines, and collect any mismatch
	// (two checks per input, so the buffer holds every possible mismatch)
	var waitGroup sync.WaitGroup
	mismatches := make(chan string, 2*64*len(sanitizeCorpus))

	for range 64 {
		waitGroup.Go(func() {
			for index, input := range sanitizeCorpus {
				document := NewDocument(input)

				if document.String() != expectedStrings[index] {
					mismatches <- "String: " + input
				}

				if document.HTMLString() != expectedHTML[index] {
					mismatches <- "HTMLString: " + input
				}
			}
		})
	}

	waitGroup.Wait()
	close(mismatches)

	for mismatch := range mismatches {
		t.Error(mismatch)
	}
}
