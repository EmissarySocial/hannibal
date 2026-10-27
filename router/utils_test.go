package router

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// TestCanTrace confirms canTrace is TRUE at Trace level and FALSE at Debug level
func TestCanTrace(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	require.True(t, canTrace())
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	require.False(t, canTrace())
}

// TestCanDebug confirms canDebug is TRUE at Trace and Debug levels and FALSE at Info level
func TestCanDebug(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	require.True(t, canDebug())
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	require.True(t, canDebug())
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	require.False(t, canDebug())
}

// TestCanInfo confirms canInfo is TRUE at Trace, Debug, and Info levels and FALSE at Warn level
func TestCanInfo(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	require.True(t, canInfo())
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	require.True(t, canInfo())
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	require.True(t, canInfo())
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	require.False(t, canInfo())
}
