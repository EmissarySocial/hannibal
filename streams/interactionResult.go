package streams

// InteractionResult is the answer an InteractionPolicy gives for one interaction
type InteractionResult int

const (
	// InteractionDenied means the interaction is not approved.  It is the zero value.
	InteractionDenied InteractionResult = iota

	// InteractionNeedsApproval means the interaction waits for the author's approval
	InteractionNeedsApproval

	// InteractionAllowed means the interaction is approved at once
	InteractionAllowed
)
