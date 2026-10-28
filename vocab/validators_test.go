package vocab

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateActivityType confirms every recognized activity type is returned
// unchanged and everything else collapses to Unknown.
func TestValidateActivityType(t *testing.T) {

	// Each recognized type must be returned verbatim. Listed against the named
	// constants so a type dropped from the switch is caught here, not in prod.
	valid := []string{
		ActivityTypeAccept,
		ActivityTypeAdd,
		ActivityTypeAnnounce,
		ActivityTypeArrive,
		ActivityTypeBlock,
		ActivityTypeCreate,
		ActivityTypeDelete,
		ActivityTypeDislike,
		ActivityTypeFlag,
		ActivityTypeFollow,
		ActivityTypeIgnore,
		ActivityTypeInvite,
		ActivityTypeJoin,
		ActivityTypeLeave,
		ActivityTypeLike,
		ActivityTypeListen,
		ActivityTypeMove,
		ActivityTypeOffer,
		ActivityTypeQuestion,
		ActivityTypeReject,
		ActivityTypeRead,
		ActivityTypeRemove,
		ActivityTypeTentativeReject,
		ActivityTypeTentativeAccept,
		ActivityTypeTravel,
		ActivityTypeUndo,
		ActivityTypeUpdate,
		ActivityTypeView,
	}

	for _, activityType := range valid {
		t.Run("valid/"+activityType, func(t *testing.T) {
			assert.Equal(t, activityType, ValidateActivityType(activityType))
		})
	}

	rejects := func(name string, input string) {
		t.Run("unknown/"+name, func(t *testing.T) {
			assert.Equal(t, Unknown, ValidateActivityType(input))
		})
	}

	rejects("empty", "")
	rejects("wrong case", "accept")
	rejects("whitespace padded", " Create ")
	rejects("garbage", "NotARealType")
	// A valid actor/object type is not a valid activity type.
	rejects("actor type", ActorTypePerson)
	rejects("object type", ObjectTypeNote)
}

// TestValidateActorType confirms every recognized actor type is returned
// unchanged and everything else collapses to Unknown.
func TestValidateActorType(t *testing.T) {

	valid := []string{
		ActorTypeApplication,
		ActorTypeGroup,
		ActorTypeOrganization,
		ActorTypePerson,
		ActorTypeService,
	}

	for _, actorType := range valid {
		t.Run("valid/"+actorType, func(t *testing.T) {
			assert.Equal(t, actorType, ValidateActorType(actorType))
		})
	}

	rejects := func(name string, input string) {
		t.Run("unknown/"+name, func(t *testing.T) {
			assert.Equal(t, Unknown, ValidateActorType(input))
		})
	}

	rejects("empty", "")
	rejects("wrong case", "person")
	rejects("garbage", "Robot")
	// A valid activity/object type is not a valid actor type.
	rejects("activity type", ActivityTypeCreate)
	rejects("object type", ObjectTypeNote)
}

// TestValidateObjectType confirms every recognized object type (including the
// Mention link type) is returned unchanged and everything else is Unknown.
func TestValidateObjectType(t *testing.T) {

	valid := []string{
		ObjectTypeArticle,
		ObjectTypeAudio,
		ObjectTypeDocument,
		ObjectTypeEvent,
		ObjectTypeImage,
		ObjectTypeNote,
		ObjectTypePage,
		ObjectTypePlace,
		ObjectTypeProfile,
		ObjectTypeRelationship,
		ObjectTypeTombstone,
		ObjectTypeVideo,
		LinkTypeMention,
	}

	for _, objectType := range valid {
		t.Run("valid/"+objectType, func(t *testing.T) {
			assert.Equal(t, objectType, ValidateObjectType(objectType))
		})
	}

	rejects := func(name string, input string) {
		t.Run("unknown/"+name, func(t *testing.T) {
			assert.Equal(t, Unknown, ValidateObjectType(input))
		})
	}

	rejects("empty", "")
	rejects("wrong case", "note")
	rejects("garbage", "Widget")
	// A valid activity/actor type is not a valid object type.
	rejects("activity type", ActivityTypeCreate)
	rejects("actor type", ActorTypePerson)
}

// FuzzValidators ensures the validators never panic and only ever return the
// input itself or the Unknown sentinel, for any input.
func FuzzValidators(f *testing.F) {

	f.Add("")
	f.Add("Create")
	f.Add("Person")
	f.Add("Note")
	f.Add("Mention")
	f.Add(" leading space")

	f.Fuzz(func(t *testing.T, input string) {

		check := func(got string) {
			// The contract: return the input verbatim, or the Unknown sentinel.
			if got != input && got != Unknown {
				t.Errorf("unexpected result %q for input %q", got, input)
			}
		}

		check(ValidateActivityType(input))
		check(ValidateActorType(input))
		check(ValidateObjectType(input))
	})
}
