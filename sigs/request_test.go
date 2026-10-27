package sigs

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHTTPRequest pins how net/http writes, reads, and indexes a header that carries several values
func TestHTTPRequest(t *testing.T) {

	request, err := http.NewRequest("GET", "http://example.com/foo", nil)
	require.Nil(t, err)

	request.Header.Add("Cache-Control", "max-age=60")
	request.Header.Add("Cache-Control", "must-revalidate")

	var buffer bytes.Buffer
	if err := request.Header.Write(&buffer); err != nil {
		panic(err)
	}

	// Guarantee that Go is writing the header the way we'd expect
	require.Equal(t, removeTabs("Cache-Control: max-age=60\r\nCache-Control: must-revalidate\r\n"), buffer.String())

	// Prove that we get the FIRST value when we call .Get()
	require.Equal(t, "max-age=60", request.Header.Get("Cache-Control"))
	require.Equal(t, "max-age=60", request.Header.Get("cAcHe-CoNtRoL"))

	// Prove that whe get ALL values when whe access via the map
	valueSlice := request.Header[http.CanonicalHeaderKey("CAcHe-CoNtroL")]
	result := strings.Join(valueSlice, ", ")

	require.Equal(t, "max-age=60, must-revalidate", result)
}
