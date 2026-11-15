package streams

import (
	"html"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/sliceof"
)

/******************************************
 * Type Detection
 ******************************************/

// DocumentCategory returns the high-level category (Actor, Activity, Object, or Collection) of the Document.
func (document Document) DocumentCategory() string {
	return DocumentCategory(document.Type())
}

// IsActivity returns TRUE if this document represents an Activity
func (document Document) IsActivity() bool {
	return IsActivity(document.Type())
}

// NotActivity returns TRUE if this document does NOT represent an Activity
func (document Document) NotActivity() bool {
	return !document.IsActivity()
}

// IsActor returns TRUE if this document represents an Actor
func (document Document) IsActor() bool {
	return IsActor(document.Type())
}

// NotActor returns TRUE if this document does NOT represent an Actor
func (document Document) NotActor() bool {
	return !document.IsActor()
}

// IsCollection returns TRUE if this document represents a Collection or CollectionPage
func (document Document) IsCollection() bool {
	return IsCollection(document.Type())
}

// NotCollection returns TRUE if the document does NOT represent a Collection or CollectionPage
func (document Document) NotCollection() bool {
	return !document.IsCollection()
}

// IsObject returns TRUE if this document represents an Object type (Article, Note, etc)
func (document Document) IsObject() bool {
	return IsObject(document.Type())
}

// NotObject returns TRUE if this document does NOT represent an Object type (Article, Note, etc)
func (document Document) NotObject() bool {
	return !document.IsObject()
}

// UnwrapActivity returns the object inside an activity (create, update, announce, etc), looking
// at most two activities deep. Any other document is returned as it is.
func (document Document) UnwrapActivity() Document {

	// If this document is actually an "Activity" type, then dig deeper
	// into its "object" property to find the actual object
	if document.IsActivity() {

		object := document.Object()

		// If the inner object is also an activity, it's OK to go one step deeper.
		// This accommodates servers like Lemmy, which send Announce > Create > Page
		if object.IsActivity() {

			// RULE: Never recurse. A remote server can send activities that contain
			// themselves, and an unbounded walk would overflow the stack.
			return object.Object()
		}

		// Order's up!
		return object
	}

	// Here 'ya go.
	return document
}

/******************************************
 * Image/Icon Metadata
 ******************************************/

// HasIcon returns TRUE if this document has a valid Icon property
func (document Document) HasIcon() bool {
	return document.Icon().NotNil()
}

// HasImage returns TRUE if this document has a valid Image property
func (document Document) HasImage() bool {
	return document.Image().NotNil()
}

// HasDimensions returns TRUE if this document has non-zero values for both "width" and "height"
func (document Document) HasDimensions() bool {
	return document.Width() > 0 && document.Height() > 0
}

// FirstImageAttachment scans all attachments and returns the first
// one with a media type that begins with "image/"
func (document Document) FirstImageAttachment() Image {

	for attachment := range document.Attachment().Range() {
		mediaType, _, _ := mime.ParseMediaType(attachment.MediaType()) // nolint:scopeguard c'mon man. scoping this below will make the one line WAAAAY too long

		if strings.HasPrefix(mediaType, "image/") {
			return NewImage(attachment.Head())
		}
	}

	return NewImage("")
}

// AspectRatio inspects a Document's "width" and "height" properties
// and (if they are non-zero) returns a computed aspect ratio
func (document Document) AspectRatio() string {

	width := document.Width()
	height := document.Height()

	if width == 0 || height == 0 {
		return "auto"
	}

	ratio := float64(width) / float64(height)
	return strconv.FormatFloat(ratio, 'f', -1, 64)
}

/******************************************
 * Content Metadata
 ******************************************/

// HasContent returns TRUE if this document has a valid Content property
func (document Document) HasContent() bool {
	return document.Content() != ""
}

// HasSummary returns TRUE if this document has a valid Summary property
func (document Document) HasSummary() bool {
	return document.Summary() != ""
}

// SummaryWithTagLinks returns the document's summary as HTML, with each "tag" that it mentions
// made into a link. The summary's own text is escaped, and only web addresses are linked.
func (document Document) SummaryWithTagLinks() string {

	summary := document.Summary()

	if summary == "" {
		return ""
	}

	links := document.tagLinks()

	// Walk the summary once. Text that has been written is never searched again, so a tag
	// cannot match inside a link, and every step moves forward.
	var result strings.Builder
	textStart := 0

	for position := 0; position < len(summary); {

		link, matchLength := longestTagLink(summary[position:], links)

		// No tag begins here, so step over one character
		if matchLength == 0 {
			_, size := utf8.DecodeRuneInString(summary[position:])
			position += size
			continue
		}

		// RULE: The summary is plain text and the result is HTML, so the text is escaped
		result.WriteString(html.EscapeString(summary[textStart:position]))
		result.WriteString(link.anchor)

		position += matchLength
		textStart = position
	}

	result.WriteString(html.EscapeString(summary[textStart:]))

	return result.String()
}

// tagLink is one tag that SummaryWithTagLinks looks for, with the link that replaces it
type tagLink struct {
	name   string // The tag's name, as it is searched for in the summary
	anchor string // The finished HTML link, already escaped
}

// tagLinks returns a link for each of the document's tags that has a name and a web address
func (document Document) tagLinks() []tagLink {

	result := make([]tagLink, 0)

	for tag := range document.Tag().Range() {

		// RULE: The tags come from another server, so only the first few are searched for
		if len(result) >= maxLinkedTags {
			break
		}

		name := tag.Name()
		href := tag.Href()

		if (name == "") || !isWebAddress(href) {
			continue
		}

		result = append(result, tagLink{
			name:   name,
			anchor: `<a href="` + html.EscapeString(href) + `" target="_blank">` + html.EscapeString(name) + `</a>`,
		})
	}

	return result
}

// isWebAddress returns TRUE if a value is an absolute http or https URL with a host
func isWebAddress(value string) bool {

	parsed, err := url.Parse(value)

	if err != nil {
		return false
	}

	// RULE: Any other scheme, such as `javascript:`, must never become a link
	if (parsed.Scheme != "http") && (parsed.Scheme != "https") {
		return false
	}

	return parsed.Host != ""
}

// longestTagLink returns the link whose name begins the text, in any letter case, and the number
// of bytes of text it matched. When several match, the longest wins. It returns zero for no match.
func longestTagLink(text string, links []tagLink) (tagLink, int) {

	var result tagLink
	longest := 0

	for _, link := range links {
		if matchLength := foldedPrefixLength(text, link.name); matchLength > longest {
			result = link
			longest = matchLength
		}
	}

	return result, longest
}

// foldedPrefixLength returns the number of bytes at the start of the text that equal the prefix
// when letter case is ignored, or zero if the text does not begin with it
func foldedPrefixLength(text string, prefix string) int {

	position := 0

	// Compare one character at a time. Two cases of a letter can take a different number of
	// bytes, so the length is counted in the text and never taken from the prefix.
	for _, expected := range prefix {

		if position >= len(text) {
			return 0
		}

		actual, size := utf8.DecodeRuneInString(text[position:])

		if !equalFoldRune(actual, expected) {
			return 0
		}

		position += size
	}

	return position
}

// equalFoldRune returns TRUE if two characters are the same letter in any case
func equalFoldRune(first rune, second rune) bool {

	if first == second {
		return true
	}

	// SimpleFold steps through every case of a letter and then returns to the start
	for folded := unicode.SimpleFold(first); folded != first; folded = unicode.SimpleFold(folded) {
		if folded == second {
			return true
		}
	}

	return false
}

/******************************************
 * Delivery Metadata
 ******************************************/

// IsPublic returns TRUE if this document is addressed to the "Public" namespace
// https://www.w3.org/TR/activitypub/#public-addressing
func (document Document) IsPublic() bool {

	public := []string{
		vocab.NamespaceActivityStreamsPublic,
		vocab.NamespaceASPublic,
		vocab.NamespacePublic,
	}

	return document.Recipients().ContainsAny(public...)
}

// NotPublic returns TRUE if this document is NOT addressed to the "Public" namespace
// https://www.w3.org/TR/activitypub/#public-addressing
func (document Document) NotPublic() bool {
	return !document.IsPublic()
}

// Recipients returns the ID of every recipient that an activity names in its to, cc, bto and
// bcc properties, in that order
func (document Document) Recipients() sliceof.String {

	result := make(sliceof.String, 0)

	for _, property := range []string{vocab.PropertyTo, vocab.PropertyCC, vocab.PropertyBTo, vocab.PropertyBCC} {
		result = append(result, document.Get(property).IDs()...)
	}

	return result
}

// PreferredInbox returns an actor's Shared Inbox (if available)
// otherwise the actor's regular Inbox
func (document Document) PreferredInbox() string {

	if sharedInbox := document.Endpoints().SharedInbox(); sharedInbox != "" {
		return sharedInbox
	}

	return document.Inbox().String()
}
