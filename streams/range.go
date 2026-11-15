package streams

import (
	"iter"
	"slices"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/sliceof"
)

// Range returns an iterator over each sub-document of the Document.
func (document Document) Range() iter.Seq[Document] {

	return func(yield func(Document) bool) {
		for ; document.NotNil(); document = document.Tail() {
			if !yield(document.Head()) {
				return
			}
		}
	}
}

// RangeWithIndex returns an iterator over each sub-document of the Document, paired with its index.
func (document Document) RangeWithIndex() iter.Seq2[int, Document] {

	return func(yield func(int, Document) bool) {

		for index := 0; document.NotNil(); document = document.Tail() {
			if !yield(index, document.Head()) {
				return
			}
			index++
		}
	}
}

// RangeIDs returns an iterator over the ID of each element of a Document: one for a single
// object, and one for each object in a list. An element with no ID is skipped. It loads nothing.
func (document Document) RangeIDs() iter.Seq[string] {

	return func(yield func(string) bool) {
		for ; document.NotNil(); document = document.Tail() {

			// RULE: An empty ID names nothing, so no caller is handed one
			id := document.Head().ID()

			if id == "" {
				continue
			}

			if !yield(id) {
				return
			}
		}
	}
}

// IDs returns the ID of each element of a Document, as RangeIDs yields them. It is the
// plural of ID, in the way that Types is the plural of Type.
func (document Document) IDs() sliceof.String {
	result := make(sliceof.String, 0, document.Len())
	return slices.AppendSeq(result, document.RangeIDs())
}

// RangeMentions returns an iterator over the IDs mentioned by the Document.
func (document Document) RangeMentions() iter.Seq[string] {
	return func(yield func(string) bool) {
		for tag := document.Tag(); tag.NotNil(); tag = tag.Tail() {

			if tag.Type() == vocab.LinkTypeMention {
				if !yield(tag.Href()) {
					return
				}
			}
		}
	}
}

// RangeAddressees returns an iterator over the addressees (to, cc, bto, bcc, audience) of the Document.
func (document Document) RangeAddressees() iter.Seq[string] {

	return joinIterators(
		document.Actor().RangeIDs(),
		document.To().RangeIDs(),
		document.CC().RangeIDs(),
		document.BTo().RangeIDs(),
		document.BCC().RangeIDs(),
		document.RangeMentions(),

		// Additional addressee sources defined by these FEPs are not yet folded in:
		// FEP-1b12 Group Federation (https://w3id.org/fep/1b12),
		// FEP-7888 the context property (https://w3id.org/fep/7888),
		// FEP-7458 the replies collection (https://w3id.org/fep/7458),
		// FEP-171b Conversation Containers (http://w3id.org/fep/171b), and
		// FEP-f228 Backfilling conversations (https://w3id.org/fep/f228).
	)
}

// RangeInReplyTo returns an iterator over the actors who should receive a reply to this Document:
// the AUTHOR of the parent document (its attributedTo) followed by the parent's own addressees.
// Yields nothing when the Document is not a reply, or the parent cannot be loaded.
func (document Document) RangeInReplyTo() iter.Seq[string] {

	const location = "hannibal.streams.Document.RangeInReplyTo"

	return func(yield func(string) bool) {

		inReplyTo := document.InReplyTo()

		if inReplyTo.IsNil() {
			return // Nothing to yield
		}

		inReplyToDocument, err := inReplyTo.Load()

		if err != nil {
			derp.Report(derp.Wrap(err, location, "Unable to load InReplyTo document", inReplyTo.ID()))
			return // Nothing to yield
		}

		// The parent's AUTHOR is the primary target — a reply notifies the person being replied to,
		// and only actors have inboxes. RangeAddressees reads `actor`/`to`/`cc`/mentions but NOT
		// `attributedTo`, so without this a reply to a Note (whose author lives in attributedTo)
		// would never reach that author.
		if author := inReplyToDocument.AttributedTo().ID(); author != "" {
			if !yield(author) {
				return // Stop yielding
			}
		}

		for address := range inReplyToDocument.RangeAddressees() {
			if !yield(address) {
				return // Stop yielding
			}
		}
	}
}
