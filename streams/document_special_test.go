package streams

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/mapof"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocument_TypeDetection confirms the Is/Not type-category predicates and
// DocumentCategory agree for one representative type from each category.
func TestDocument_TypeDetection(t *testing.T) {

	// withType builds a document carrying only the given "type" property.
	withType := func(documentType string) Document {
		return NewDocument(map[string]any{vocab.PropertyType: documentType})
	}

	t.Run("activity", func(t *testing.T) {
		doc := withType(vocab.ActivityTypeCreate)
		assert.Equal(t, vocab.DocumentCategoryActivity, doc.DocumentCategory())
		assert.True(t, doc.IsActivity())
		assert.False(t, doc.NotActivity())
		assert.False(t, doc.IsObject())
		assert.True(t, doc.NotObject())
	})

	t.Run("actor", func(t *testing.T) {
		doc := withType(vocab.ActorTypePerson)
		assert.Equal(t, vocab.DocumentCategoryActor, doc.DocumentCategory())
		assert.True(t, doc.IsActor())
		assert.False(t, doc.NotActor())
	})

	t.Run("collection", func(t *testing.T) {
		doc := withType(vocab.CoreTypeOrderedCollection)
		assert.Equal(t, vocab.DocumentCategoryCollection, doc.DocumentCategory())
		assert.True(t, doc.IsCollection())
		assert.False(t, doc.NotCollection())
	})

	t.Run("object", func(t *testing.T) {
		doc := withType(vocab.ObjectTypeNote)
		assert.Equal(t, vocab.DocumentCategoryObject, doc.DocumentCategory())
		assert.True(t, doc.IsObject())
		assert.False(t, doc.NotObject())
	})

	t.Run("unknown -> object", func(t *testing.T) {
		// An unrecognized type falls through to the Object category.
		doc := withType("SomethingMadeUp")
		assert.Equal(t, vocab.DocumentCategoryObject, doc.DocumentCategory())
		assert.False(t, doc.IsActivity())
		assert.False(t, doc.IsActor())
		assert.False(t, doc.IsCollection())
	})
}

// TestDocument_UnwrapActivity confirms that activities unwrap to the object inside them, at most
// two activities deep, while a non-activity document is returned unchanged.
func TestDocument_UnwrapActivity(t *testing.T) {

	t.Run("an activity unwraps to its object", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyType: vocab.ActivityTypeCreate,
			vocab.PropertyObject: map[string]any{
				vocab.PropertyType: vocab.ObjectTypeNote,
				vocab.PropertyID:   "urn:note",
			},
		})

		unwrapped := doc.UnwrapActivity()
		assert.Equal(t, vocab.ObjectTypeNote, unwrapped.Type())
		assert.Equal(t, "urn:note", unwrapped.ID())
	})

	t.Run("an Announce of a Create unwraps to the object", func(t *testing.T) {
		// Announce > Create > Note, as produced by some servers (e.g. Lemmy).
		doc := NewDocument(map[string]any{
			vocab.PropertyType: vocab.ActivityTypeAnnounce,
			vocab.PropertyObject: map[string]any{
				vocab.PropertyType: vocab.ActivityTypeCreate,
				vocab.PropertyObject: map[string]any{
					vocab.PropertyType: vocab.ObjectTypeNote,
					vocab.PropertyID:   "urn:note",
				},
			},
		})

		unwrapped := doc.UnwrapActivity()
		assert.Equal(t, vocab.ObjectTypeNote, unwrapped.Type())
		assert.Equal(t, "urn:note", unwrapped.ID())
	})

	t.Run("unwrapping stops two activities deep", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyType: vocab.ActivityTypeAnnounce,
			vocab.PropertyObject: map[string]any{
				vocab.PropertyType: vocab.ActivityTypeUndo,
				vocab.PropertyObject: map[string]any{
					vocab.PropertyType:   vocab.ActivityTypeLike,
					vocab.PropertyID:     "urn:like",
					vocab.PropertyObject: "urn:note",
				},
			},
		})

		unwrapped := doc.UnwrapActivity()
		assert.Equal(t, vocab.ActivityTypeLike, unwrapped.Type())
		assert.Equal(t, "urn:like", unwrapped.ID())
	})

	t.Run("non-activity returns itself", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyType: vocab.ObjectTypeNote,
			vocab.PropertyID:   "urn:note",
		})
		assert.Equal(t, "urn:note", doc.UnwrapActivity().ID())
	})
}

// TestDocument_UnwrapActivity_Loaded confirms that activities loaded by URL unwrap the same way,
// and that activities which contain themselves return instead of recursing.
func TestDocument_UnwrapActivity_Loaded(t *testing.T) {

	const (
		announceURL = "https://social.example.com/activities/announce"
		createURL   = "https://social.example.com/activities/create"
		pageURL     = "https://social.example.com/pages/1"
		selfURL     = "https://social.example.com/activities/self"
		loopAURL    = "https://social.example.com/activities/a"
		loopBURL    = "https://social.example.com/activities/b"
		cycleURL    = "https://social.example.com/activities/cycle-announce"
		cycleInner  = "https://social.example.com/activities/cycle-create"
	)

	client := testClient{data: mapof.Any{
		announceURL: map[string]any{vocab.PropertyID: announceURL, vocab.PropertyType: vocab.ActivityTypeAnnounce, vocab.PropertyObject: createURL},
		createURL:   map[string]any{vocab.PropertyID: createURL, vocab.PropertyType: vocab.ActivityTypeCreate, vocab.PropertyObject: pageURL},
		pageURL:     map[string]any{vocab.PropertyID: pageURL, vocab.PropertyType: vocab.ObjectTypePage},
		selfURL:     map[string]any{vocab.PropertyID: selfURL, vocab.PropertyType: vocab.ActivityTypeCreate, vocab.PropertyObject: selfURL},
		loopAURL:    map[string]any{vocab.PropertyID: loopAURL, vocab.PropertyType: vocab.ActivityTypeCreate, vocab.PropertyObject: loopBURL},
		loopBURL:    map[string]any{vocab.PropertyID: loopBURL, vocab.PropertyType: vocab.ActivityTypeCreate, vocab.PropertyObject: loopAURL},
		cycleURL:    map[string]any{vocab.PropertyID: cycleURL, vocab.PropertyType: vocab.ActivityTypeAnnounce, vocab.PropertyObject: cycleInner},
		cycleInner:  map[string]any{vocab.PropertyID: cycleInner, vocab.PropertyType: vocab.ActivityTypeCreate, vocab.PropertyObject: cycleURL},
	}}

	t.Run("an Announce of a Create unwraps to the object", func(t *testing.T) {
		unwrapped := unwrapWithin(t, client, announceURL)
		assert.Equal(t, pageURL, unwrapped.ID())
		assert.Equal(t, vocab.ObjectTypePage, unwrapped.Type())
	})

	t.Run("a Create of itself returns its own URL", func(t *testing.T) {
		unwrapped := unwrapWithin(t, client, selfURL)
		assert.Equal(t, selfURL, unwrapped.Value())
	})

	t.Run("two Creates of each other return the first URL", func(t *testing.T) {
		unwrapped := unwrapWithin(t, client, loopAURL)
		assert.Equal(t, loopAURL, unwrapped.Value())
	})

	t.Run("an Announce of a Create of the Announce returns the Announce URL", func(t *testing.T) {
		unwrapped := unwrapWithin(t, client, cycleURL)
		assert.Equal(t, cycleURL, unwrapped.Value())
	})
}

// unwrapWithin loads the document at a URL and unwraps it, failing the test if the unwrap
// does not return within five seconds.
func unwrapWithin(t *testing.T, client testClient, url string) Document {

	t.Helper()

	loaded, err := client.Load(url)
	require.NoError(t, err)

	done := make(chan Document, 1)

	go func() {
		done <- loaded.UnwrapActivity()
	}()

	select {
	case result := <-done:
		return result

	case <-time.After(5 * time.Second):
		require.FailNow(t, "UnwrapActivity did not return", url)
		return NilDocument()
	}
}

// TestDocument_ImageMetadata exercises the icon/image presence and dimension
// helpers.
func TestDocument_ImageMetadata(t *testing.T) {

	t.Run("has icon and image", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyIcon:  map[string]any{vocab.PropertyURL: "https://example.com/icon.png"},
			vocab.PropertyImage: map[string]any{vocab.PropertyURL: "https://example.com/image.png"},
		})
		assert.True(t, doc.HasIcon())
		assert.True(t, doc.HasImage())
	})

	t.Run("missing icon and image", func(t *testing.T) {
		doc := NewDocument(map[string]any{})
		assert.False(t, doc.HasIcon())
		assert.False(t, doc.HasImage())
	})

	t.Run("has dimensions", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyWidth:  640,
			vocab.PropertyHeight: 480,
		})
		assert.True(t, doc.HasDimensions())
	})

	t.Run("missing one dimension", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyWidth: 640})
		assert.False(t, doc.HasDimensions())
	})
}

// TestDocument_AspectRatio confirms the aspect ratio is computed for valid
// dimensions and falls back to "auto" when either dimension is zero.
func TestDocument_AspectRatio(t *testing.T) {

	t.Run("square", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyWidth: 100, vocab.PropertyHeight: 100})
		assert.Equal(t, "1", doc.AspectRatio())
	})

	t.Run("two-to-one", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyWidth: 200, vocab.PropertyHeight: 100})
		assert.Equal(t, "2", doc.AspectRatio())
	})

	t.Run("zero height -> auto", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyWidth: 200})
		assert.Equal(t, "auto", doc.AspectRatio())
	})

	t.Run("zero width -> auto", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyHeight: 200})
		assert.Equal(t, "auto", doc.AspectRatio())
	})
}

// TestDocument_FirstImageAttachment confirms the scan returns the first
// image/* attachment and ignores non-image attachments.
func TestDocument_FirstImageAttachment(t *testing.T) {

	t.Run("returns first image attachment", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyAttachment: []any{
				map[string]any{vocab.PropertyMediaType: "application/pdf", vocab.PropertyURL: "https://example.com/doc.pdf"},
				map[string]any{vocab.PropertyMediaType: "image/png", vocab.PropertyURL: "https://example.com/pic.png"},
			},
		})
		assert.Equal(t, "https://example.com/pic.png", doc.FirstImageAttachment().URL())
	})

	t.Run("no image attachment -> nil image", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyAttachment: []any{
				map[string]any{vocab.PropertyMediaType: "application/pdf", vocab.PropertyURL: "https://example.com/doc.pdf"},
			},
		})
		assert.False(t, doc.FirstImageAttachment().NotNil())
	})
}

// TestDocument_IconImageHybrids confirms the order in which IconOrImage (icon, image, attachment)
// and ImageOrIcon (image, attachment, icon) fall back, and that each returns a nil Image when all are missing.
func TestDocument_IconImageHybrids(t *testing.T) {

	icon := map[string]any{vocab.PropertyURL: "https://example.com/icon.png"}
	image := map[string]any{vocab.PropertyURL: "https://example.com/image.png"}
	attachment := []any{
		map[string]any{vocab.PropertyMediaType: "image/png", vocab.PropertyURL: "https://example.com/attachment.png"},
	}

	t.Run("IconOrImage prefers image over attachment", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyImage: image, vocab.PropertyAttachment: attachment})
		assert.Equal(t, "https://example.com/image.png", doc.IconOrImage().URL())
	})

	t.Run("IconOrImage falls back to attachment", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyAttachment: attachment})
		assert.Equal(t, "https://example.com/attachment.png", doc.IconOrImage().URL())
	})

	t.Run("IconOrImage with nothing -> nil image", func(t *testing.T) {
		assert.True(t, NewDocument(map[string]any{}).IconOrImage().IsNil())
	})

	t.Run("ImageOrIcon prefers attachment over icon", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyIcon: icon, vocab.PropertyAttachment: attachment})
		assert.Equal(t, "https://example.com/attachment.png", doc.ImageOrIcon().URL())
	})

	t.Run("ImageOrIcon with nothing -> nil image", func(t *testing.T) {
		assert.True(t, NewDocument(map[string]any{}).ImageOrIcon().IsNil())
	})

	t.Run("IconOrImage prefers icon", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyIcon: icon, vocab.PropertyImage: image})
		assert.Equal(t, "https://example.com/icon.png", doc.IconOrImage().URL())
	})

	t.Run("IconOrImage falls back to image", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyImage: image})
		assert.Equal(t, "https://example.com/image.png", doc.IconOrImage().URL())
	})

	t.Run("ImageOrIcon prefers image", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyIcon: icon, vocab.PropertyImage: image})
		assert.Equal(t, "https://example.com/image.png", doc.ImageOrIcon().URL())
	})

	t.Run("ImageOrIcon falls back to icon", func(t *testing.T) {
		doc := NewDocument(map[string]any{vocab.PropertyIcon: icon})
		assert.Equal(t, "https://example.com/icon.png", doc.ImageOrIcon().URL())
	})
}

// TestDocument_ContentMetadata exercises the content/summary presence helpers.
func TestDocument_ContentMetadata(t *testing.T) {

	withContent := NewDocument(map[string]any{
		vocab.PropertyContent: "Hello, world",
		vocab.PropertySummary: "A greeting",
	})
	assert.True(t, withContent.HasContent())
	assert.True(t, withContent.HasSummary())

	empty := NewDocument(map[string]any{})
	assert.False(t, empty.HasContent())
	assert.False(t, empty.HasSummary())
}

// TestDocument_SummaryWithTagLinks confirms that tag mentions in the summary are
// rewritten as HTML links, and that an empty summary is returned unchanged.
func TestDocument_SummaryWithTagLinks(t *testing.T) {

	t.Run("rewrites matching tag", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertySummary: "Hello #golang fans",
			vocab.PropertyTag: map[string]any{
				vocab.PropertyName: "#golang",
				vocab.PropertyHref: "https://example.com/tags/golang",
			},
		})

		expected := `Hello <a href="https://example.com/tags/golang" target="_blank">#golang</a> fans`
		assert.Equal(t, expected, doc.SummaryWithTagLinks())
	})

	t.Run("empty summary -> empty string", func(t *testing.T) {
		doc := NewDocument(map[string]any{})
		assert.Equal(t, "", doc.SummaryWithTagLinks())
	})

	t.Run("tag without href is skipped", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertySummary: "Hello #golang fans",
			vocab.PropertyTag: map[string]any{
				vocab.PropertyName: "#golang",
			},
		})
		assert.Equal(t, "Hello #golang fans", doc.SummaryWithTagLinks())
	})
}

// tagLinkDocument returns a document with a summary and one tag for each name/href pair
func tagLinkDocument(summary string, namesAndHrefs ...string) Document {

	tags := make([]any, 0, len(namesAndHrefs)/2)

	for index := 0; index+1 < len(namesAndHrefs); index += 2 {
		tags = append(tags, map[string]any{
			vocab.PropertyName: namesAndHrefs[index],
			vocab.PropertyHref: namesAndHrefs[index+1],
		})
	}

	return NewDocument(map[string]any{
		vocab.PropertySummary: summary,
		vocab.PropertyTag:     tags,
	})
}

// TestDocument_SummaryWithTagLinks_RepeatedTag pins BUG: a tag named twice in the summary
// used to loop forever, because the search position was relative and was used as absolute
func TestDocument_SummaryWithTagLinks_RepeatedTag(t *testing.T) {

	doc := tagLinkDocument("hello #a and #a again", "#a", "https://example.com/t")
	link := `<a href="https://example.com/t" target="_blank">#a</a>`

	require.Equal(t, "hello "+link+" and "+link+" again", doc.SummaryWithTagLinks())
}

// TestDocument_SummaryWithTagLinks_CaseFolding confirms that a tag matches in any letter case,
// including letters whose two cases take a different number of bytes
func TestDocument_SummaryWithTagLinks_CaseFolding(t *testing.T) {

	// Plain ASCII, in a different case
	doc := tagLinkDocument("Hello #GoLang fans", "#golang", "https://example.com/t")
	require.Equal(t, `Hello <a href="https://example.com/t" target="_blank">#golang</a> fans`, doc.SummaryWithTagLinks())

	// The Kelvin sign is three bytes and folds to "k", which is one. This used to panic.
	doc = tagLinkDocument("k", "\u212a", "https://example.com/x")
	require.Equal(t, `<a href="https://example.com/x" target="_blank">`+"\u212a"+`</a>`, doc.SummaryWithTagLinks())

	doc = tagLinkDocument("a \u212a b", "k", "https://example.com/x")
	require.Equal(t, `a <a href="https://example.com/x" target="_blank">k</a> b`, doc.SummaryWithTagLinks())
}

// TestDocument_SummaryWithTagLinks_EscapesText confirms that the summary's own text is escaped,
// because the result is HTML and the summary is plain text
func TestDocument_SummaryWithTagLinks_EscapesText(t *testing.T) {

	doc := tagLinkDocument("hi #tag &lt;img src=x onerror=alert(1)&gt;", "#tag", "https://example.com/t")

	expected := `hi <a href="https://example.com/t" target="_blank">#tag</a> &lt;img src=x onerror=alert(1)&gt;`
	require.Equal(t, expected, doc.SummaryWithTagLinks())
}

// TestDocument_SummaryWithTagLinks_EscapesLink confirms that a tag cannot break out of the
// link's attribute or its text
func TestDocument_SummaryWithTagLinks_EscapesLink(t *testing.T) {

	doc := tagLinkDocument(`see a"b here`, `a"b`, `https://example.com/t?x=1&y="2"`)

	expected := `see <a href="https://example.com/t?x=1&amp;y=&#34;2&#34;" target="_blank">a&#34;b</a> here`
	require.Equal(t, expected, doc.SummaryWithTagLinks())
}

// TestDocument_SummaryWithTagLinks_UnsafeHref confirms that only a web address becomes a link
func TestDocument_SummaryWithTagLinks_UnsafeHref(t *testing.T) {

	for _, href := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)//",
		"data:text/html,<script>alert(1)</script>",
		"vbscript:x",
		"//example.com/no-scheme",
		"/relative",
		"https://",
		"not a url",
		"http://exa mple.com/",
	} {
		doc := tagLinkDocument("hello #tag", "#tag", href)
		require.Equal(t, "hello #tag", doc.SummaryWithTagLinks(), "href: %s", href)
	}
}

// TestDocument_SummaryWithTagLinks_LongestTagWins confirms that when two tags match at the
// same place, the longer one is linked
func TestDocument_SummaryWithTagLinks_LongestTagWins(t *testing.T) {

	doc := tagLinkDocument("I like #golang", "#go", "https://example.com/go", "#golang", "https://example.com/golang")
	require.Equal(t, `I like <a href="https://example.com/golang" target="_blank">#golang</a>`, doc.SummaryWithTagLinks())

	// The shorter tag still links where the longer one does not match
	doc = tagLinkDocument("#go and #golang", "#go", "https://example.com/go", "#golang", "https://example.com/golang")
	expected := `<a href="https://example.com/go" target="_blank">#go</a> and <a href="https://example.com/golang" target="_blank">#golang</a>`
	require.Equal(t, expected, doc.SummaryWithTagLinks())
}

// TestDocument_SummaryWithTagLinks_NeverRelinksALink confirms that a tag whose name appears
// inside another tag's link is not linked there
func TestDocument_SummaryWithTagLinks_NeverRelinksALink(t *testing.T) {

	doc := tagLinkDocument("#first", "#first", "https://example.com/href", "href", "https://example.com/second", "a", "https://example.com/third")
	require.Equal(t, `<a href="https://example.com/href" target="_blank">#first</a>`, doc.SummaryWithTagLinks())
}

// TestDocument_SummaryWithTagLinks_TagLimit confirms that tags past the limit are left as text
func TestDocument_SummaryWithTagLinks_TagLimit(t *testing.T) {

	namesAndHrefs := make([]string, 0, 2*(maxLinkedTags+1))

	for index := range maxLinkedTags {
		namesAndHrefs = append(namesAndHrefs, "#filler"+strconv.Itoa(index)+"x", "https://example.com/filler")
	}

	namesAndHrefs = append(namesAndHrefs, "#last", "https://example.com/last")

	doc := tagLinkDocument("#filler0x then #last", namesAndHrefs...)
	require.Equal(t, `<a href="https://example.com/filler" target="_blank">#filler0x</a> then #last`, doc.SummaryWithTagLinks())
}

// TestDocument_SummaryWithTagLinks_OddInput confirms that empty names and broken text do no harm
func TestDocument_SummaryWithTagLinks_OddInput(t *testing.T) {

	// A tag with no name matches nothing
	doc := tagLinkDocument("hello", "", "https://example.com/t")
	require.Equal(t, "hello", doc.SummaryWithTagLinks())

	// Bytes that are not valid UTF-8 pass through, and the scan still ends
	doc = tagLinkDocument("a\xffb #t", "#t", "https://example.com/t")
	require.Equal(t, "a\xffb "+`<a href="https://example.com/t" target="_blank">#t</a>`, doc.SummaryWithTagLinks())
}

// TestDocument_PublicAddressing confirms IsPublic / NotPublic detect each of the three forms of
// the Public namespace, in each of the four recipient fields.
func TestDocument_PublicAddressing(t *testing.T) {

	t.Run("public in 'to'", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyTo: vocab.NamespaceActivityStreamsPublic,
		})
		assert.True(t, doc.IsPublic())
		assert.False(t, doc.NotPublic())
	})

	// isPublic asserts that a document addressing this value through this field is public.
	isPublic := func(field string, value any) {
		t.Run("public in '"+field+"'", func(t *testing.T) {
			doc := NewDocument(map[string]any{field: value})
			assert.True(t, doc.IsPublic(), "%s: %v", field, value)
		})
	}

	isPublic(vocab.PropertyCC, vocab.NamespaceActivityStreamsPublic)
	isPublic(vocab.PropertyBTo, vocab.NamespaceActivityStreamsPublic)
	isPublic(vocab.PropertyBCC, vocab.NamespaceActivityStreamsPublic)
	isPublic(vocab.PropertyTo, vocab.NamespaceASPublic)
	isPublic(vocab.PropertyTo, vocab.NamespacePublic)
	isPublic(vocab.PropertyTo, []any{"https://example.com/users/alice", vocab.NamespaceASPublic})

	t.Run("not public", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyTo: "https://example.com/users/alice",
		})
		assert.False(t, doc.IsPublic())
		assert.True(t, doc.NotPublic())
	})
}

// TestDocument_Recipients confirms recipients are collected from to/cc/bto/bcc.
func TestDocument_Recipients(t *testing.T) {

	doc := NewDocument(map[string]any{
		vocab.PropertyTo:  "https://example.com/users/alice",
		vocab.PropertyCC:  "https://example.com/users/bob",
		vocab.PropertyBTo: "https://example.com/users/carol",
		vocab.PropertyBCC: "https://example.com/users/dave",
	})

	recipients := doc.Recipients()
	assert.Contains(t, recipients, "https://example.com/users/alice")
	assert.Contains(t, recipients, "https://example.com/users/bob")
	assert.Contains(t, recipients, "https://example.com/users/carol")
	assert.Contains(t, recipients, "https://example.com/users/dave")
	assert.Len(t, recipients, 4)
}

// TestDocument_PreferredInbox confirms the shared inbox is preferred when
// present, otherwise the actor's regular inbox is returned.
func TestDocument_PreferredInbox(t *testing.T) {

	t.Run("shared inbox preferred", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyInbox: "https://example.com/users/alice/inbox",
			vocab.PropertyEndpoints: map[string]any{
				vocab.EndpointSharedInbox: "https://example.com/inbox",
			},
		})
		assert.Equal(t, "https://example.com/inbox", doc.PreferredInbox())
	})

	t.Run("falls back to regular inbox", func(t *testing.T) {
		doc := NewDocument(map[string]any{
			vocab.PropertyInbox: "https://example.com/users/alice/inbox",
		})
		assert.Equal(t, "https://example.com/users/alice/inbox", doc.PreferredInbox())
	})
}

// TestDocument_HTTPHeader confirms the HTTP header round-trips through the
// getter and setter.
func TestDocument_HTTPHeader(t *testing.T) {

	doc := NewDocument(map[string]any{})
	assert.Empty(t, doc.HTTPHeader())

	header := http.Header{}
	header.Set("Content-Type", vocab.ContentTypeActivityPub)
	doc.SetHTTPHeader(header)

	assert.Equal(t, vocab.ContentTypeActivityPub, doc.HTTPHeader().Get("Content-Type"))
}
