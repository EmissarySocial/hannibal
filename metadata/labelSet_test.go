package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLabelSet_Empty confirms a nil/zero set reads as clean.
func TestLabelSet_Empty(t *testing.T) {

	var set LabelSet

	assert.False(t, set.IsHidden())
	assert.False(t, set.HasAnnotations())
	assert.Equal(t, "", set.Reason())
	assert.Empty(t, set.Hidden())
	assert.Empty(t, set.Annotations())
}

// TestLabelSet_HiddenAndAnnotations confirms the split accessors and that Reason() names the first
// hiding Label.
func TestLabelSet_HiddenAndAnnotations(t *testing.T) {

	set := LabelSet{
		{Value: "Blocked by server policy", IsHidden: true},
		{Value: "Muted by your rules", IsHidden: true},
		{Value: "Politics", IsHidden: false},
	}

	assert.True(t, set.IsHidden())
	assert.True(t, set.HasAnnotations())
	assert.Equal(t, "Blocked by server policy", set.Reason())

	assert.Equal(t, LabelSet{set[0], set[1]}, set.Hidden())
	assert.Len(t, set.Annotations(), 1)
	assert.Equal(t, "Politics", set.Annotations()[0].Value)
}

// TestLabelSet_AnnotationsOnly confirms a label-only set never hides.
func TestLabelSet_AnnotationsOnly(t *testing.T) {

	set := LabelSet{
		{Value: "Politics"},
		{Value: "Sports"},
	}

	assert.False(t, set.IsHidden())
	assert.True(t, set.HasAnnotations())
	assert.Equal(t, "", set.Reason())
	assert.Equal(t, set, set.Annotations())
}

// TestLabelSet_Clone confirms a clone owns its own backing array.
func TestLabelSet_Clone(t *testing.T) {

	original := LabelSet{{Value: "Muted", IsHidden: true}}
	clone := original.Clone()
	assert.Equal(t, original, clone)
	require.Equal(t, 1, len(clone), "clone should have exactly one Label")

	// Writing through the clone must not reach the original.
	clone[0].Value = "changed"
	assert.Equal(t, "Muted", original[0].Value)
}
