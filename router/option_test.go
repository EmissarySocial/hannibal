package router

import (
	"testing"

	"github.com/benpate/hannibal/validator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewReceiveConfig_Defaults confirms the default config ships with a
// validator chain (HTTP signature + deleted-object checks).
func TestNewReceiveConfig_Defaults(t *testing.T) {

	config := NewReceiveConfig()
	require.NotEmpty(t, config.Validators)

	// The default chain must include an HTTPSig validator.
	hasHTTPSig := false
	for _, v := range config.Validators {
		if _, ok := v.(validator.HTTPSig); ok {
			hasHTTPSig = true
		}
	}
	assert.True(t, hasHTTPSig, "default chain should include an HTTPSig validator")
}

// TestWithValidators confirms the option replaces the entire validator chain.
func TestWithValidators(t *testing.T) {

	custom := stubValidator{validator.ResultValid}
	config := NewReceiveConfig(WithValidators(custom))

	require.Len(t, config.Validators, 1)
	assert.Equal(t, custom, config.Validators[0])
}

// TestWithPublicKeyFinder confirms the option swaps the default HTTPSig validator
// for one bound to the provided key finder, leaving the rest of the chain intact.
func TestWithPublicKeyFinder(t *testing.T) {

	defaultCount := len(NewReceiveConfig().Validators)

	keyFinder := func(_ string) (string, error) { return "", nil }
	config := NewReceiveConfig(WithPublicKeyFinder(keyFinder))

	// The chain length is unchanged -- the HTTPSig validator is replaced in place.
	assert.Len(t, config.Validators, defaultCount)

	// An HTTPSig validator is still present.
	hasHTTPSig := false
	for _, v := range config.Validators {
		if _, ok := v.(validator.HTTPSig); ok {
			hasHTTPSig = true
		}
	}
	assert.True(t, hasHTTPSig)
}
