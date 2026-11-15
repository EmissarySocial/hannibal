package streams

import (
	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/convert"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
)

// Image represents an ActivityStreams Image object (a string, map, or slice value).
// https://www.w3.org/ns/activitystreams#Image
type Image struct {
	value any
}

// NewImage creates a new Image object from a JSON-LD value (string, map[string]any, or []any)
func NewImage(value any) Image {

	// NILCHECK: allow `nil` values to become empty images
	if value == nil {
		return Image{}
	}

	switch typed := value.(type) {

	case Document:
		return NewImage(typed.getValue().Raw())

	case property.Value:
		return NewImage(typed.Raw())

	case Image:
		return typed

	case string:
		return Image{value: typed}

	case map[string]any:
		return Image{value: typed}

	case []any:
		return Image{value: typed}

	case mapof.Any:
		return Image{value: map[string]any(typed)}

	case sliceof.Any:
		return Image{value: []any(typed)}
	}

	return Image{""}
}

// URL returns the Image's URL property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-href
// Note: URL is an alias for Href, which is the proper name to use
func (image Image) URL() string {
	return image.Href()
}

// Href returns the Image's address when it is an absolute http or https URL, and an empty
// string otherwise. https://www.w3.org/TR/activitystreams-vocabulary/#dfn-href
func (image Image) Href() string {

	// RULE: The address comes from another server and is written into pages, so any other
	// scheme, such as `javascript:`, is never returned
	if href := image.rawHref(); isWebAddress(href) {
		return href
	}

	return ""
}

// rawHref returns the Image's address as the document wrote it, from "href" and then "url"
func (image Image) rawHref() string {

	switch typed := image.value.(type) {

	case string:
		return typed

	case map[string]any:

		if href := convert.String(typed[vocab.PropertyHref]); href != "" {
			return href
		}

		if url := convert.String(typed[vocab.PropertyURL]); url != "" {

			return url
		}

	case []any:
		if len(typed) > 0 {
			return NewImage(typed[0]).rawHref()
		}
	}

	return ""
}

// Summary returns the Image's Summary property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-summary
func (image Image) Summary() string {

	switch typed := image.value.(type) {

	case map[string]any:
		return plainText(convert.String(typed[vocab.PropertySummary]))

	case []any:
		if len(typed) > 0 {
			return NewImage(typed[0]).Summary()
		}
	}

	return ""
}

// MediaType returns the Image's MediaType property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-mediatype
func (image Image) MediaType() string {

	switch typed := image.value.(type) {

	case map[string]any:
		return plainText(convert.String(typed[vocab.PropertyMediaType]))

	case []any:
		if len(typed) > 0 {
			return NewImage(typed[0]).MediaType()
		}
	}

	return ""
}

// Height returns the Image's Height property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-height
func (image Image) Height() int {

	switch typed := image.value.(type) {

	case map[string]any:
		return convert.Int(typed[vocab.PropertyHeight])

	case []any:
		if len(typed) > 0 {
			return NewImage(typed[0]).Height()
		}
	}

	return 0
}

// Width returns the Image's Width property.
// https://www.w3.org/TR/activitystreams-vocabulary/#dfn-width
func (image Image) Width() int {

	switch typed := image.value.(type) {

	case map[string]any:
		return convert.Int(typed[vocab.PropertyWidth])

	case []any:
		if len(typed) > 0 {
			return NewImage(typed[0]).Width()
		}
	}

	return 0
}

// IsNil returns TRUE if this image is nil (having no URL)
func (image Image) IsNil() bool {
	return image.URL() == ""
}

// NotNil returns TRUE if this image has a URL
func (image Image) NotNil() bool {
	return !image.IsNil()
}

// HasHeight returns TRUE if this image has a height defined
func (image Image) HasHeight() bool {
	return image.Height() > 0
}

// HasWidth returns TRUE if this image has a width defined
func (image Image) HasWidth() bool {
	return image.Width() > 0
}

// HasDimensions returns TRUE if this image has both a height and width defined
func (image Image) HasDimensions() bool {
	return image.HasHeight() && image.HasWidth()
}

// AspectRatio calculates the aspect ratio of the image (width / height)
// If height and width are not available, then 0 is returned
func (image Image) AspectRatio() float64 {
	if image.HasDimensions() {
		return float64(image.Width()) / float64(image.Height())
	}

	return 0
}
