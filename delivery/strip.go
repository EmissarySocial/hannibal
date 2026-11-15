package delivery

import (
	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/deep"
)

// StripBlindRecipients returns a deep copy of a document with `bto` and `bcc` removed at every
// level of nesting. The original is not changed, and the copy keeps the type of every value.
func StripBlindRecipients(document map[string]any) (map[string]any, error) {

	const location = "hannibal.delivery.StripBlindRecipients"

	// RULE: The strip works on a copy that shares nothing with the original, because the
	// fan-out and the stored document still need their blind recipients
	result, err := deep.Copy(document)

	if err != nil {
		return nil, derp.Wrap(err, location, "Unable to copy document")
	}

	// A nil document is sent as an empty object
	if result == nil {
		return make(map[string]any), nil
	}

	// RULE: A value that cannot be searched is an error, and nothing is returned to send,
	// because a blind recipient hidden inside it would go out with the activity
	if err := deep.Remove(result, vocab.PropertyBTo, vocab.PropertyBCC); err != nil {
		return nil, derp.Wrap(err, location, "Unable to remove blind recipients")
	}

	// Nothing to see here
	return result, nil
}
