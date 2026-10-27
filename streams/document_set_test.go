package streams

import (
	"testing"

	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/require"
)

// TestSetProperty confirms that SetProperty writes a value into a map-backed document.
func TestSetProperty(t *testing.T) {
	document := NewDocument(mapof.NewAny())
	document.SetProperty("test-property", "test-value")
	require.Equal(t, "test-value", document.Get("test-property").Value())
}

// TestSetID confirms that SetID reports success and that ID reads the new value back.
func TestSetID(t *testing.T) {
	document := NewDocument(mapof.NewAny())
	success := document.SetID("test-id")
	require.True(t, success)
	require.Equal(t, "test-id", document.ID())
}

// TestSetID_Nested confirms that SetID on a document and on its embedded object each write through
// to the shared value.
func TestSetID_Nested(t *testing.T) {

	value := mapof.Any{
		"id": "original-id",
		"object": mapof.Any{
			"id": "original-id",
		},
	}
	document := NewDocument(value)
	success := document.SetID("new-id")

	document.Object().SetID("new-id")
	require.True(t, success)
	require.Equal(t, "new-id", document.ID())
	require.Equal(t, "new-id", document.Object().ID())
}
