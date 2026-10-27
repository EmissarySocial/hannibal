package validator

import (
	"testing"

	"github.com/benpate/hannibal/streams"
	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/assert"
)

// TestDeletedObject_NotDelete confirms the DeletedObject validator only applies
// to Delete activities; everything else is Unknown (it abstains, no network).
func TestDeletedObject_NotDelete(t *testing.T) {

	v := NewDeletedObject()

	check := func(activityType string) {
		t.Run(activityType, func(t *testing.T) {
			activity := streams.NewDocument(map[string]any{vocab.PropertyType: activityType})
			assert.Equal(t, ResultUnknown, v.Validate(blankRequest(), &activity))
		})
	}

	check(vocab.ActivityTypeCreate)
	check(vocab.ActivityTypeUpdate)
	check(vocab.ActivityTypeFollow)
	check(vocab.ActivityTypeLike)
}

// TestDeletedObject_DeleteWithoutObject confirms a Delete activity with no object
// ID is invalid (there is nothing whose deletion we could confirm), without any
// network lookup.
func TestDeletedObject_DeleteWithoutObject(t *testing.T) {

	v := NewDeletedObject()
	activity := streams.NewDocument(map[string]any{
		vocab.PropertyType: vocab.ActivityTypeDelete,
	})

	assert.Equal(t, ResultInvalid, v.Validate(blankRequest(), &activity))
}
