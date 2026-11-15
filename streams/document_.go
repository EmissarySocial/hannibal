package streams

import (
	"net/http"
	"time"

	"github.com/benpate/derp"
	"github.com/benpate/hannibal/metadata"
	"github.com/benpate/hannibal/property"
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/convert"
	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/benpate/uri"
)

// Document represents a single ActivityStream document
// or document fragment.  Due to the flexibility of ActivityStreams
// (and JSON-LD), this may be a data structure such as a
// `map[string]any`, `[]any`, or a primitive type, like a
// `string`, `float`, `int` or `bool`.
type Document struct {
	value      property.Value
	Metadata   metadata.Metadata
	httpHeader http.Header
	client     Client
}

// NewDocument creates a new Document object from a JSON-LD map[string]any. It loads nothing
// from the network unless WithClient or WithDefaultClient gives it a Client.
func NewDocument(value any, options ...DocumentOption) Document {

	result := Document{
		value:      property.NewValue(value),
		Metadata:   metadata.New(),
		httpHeader: make(http.Header),
		client:     NewNilClient(),
	}

	result.WithOptions(options...)
	return result
}

// NilDocument returns a new, empty Document.
func NilDocument(options ...DocumentOption) Document {
	return NewDocument(nil, options...)
}

// getValue returns the document's value. A Document made without NewDocument, such as a
// struct field that the JSON left out, holds no value, and reads as nil.
func (document Document) getValue() property.Value {

	if document.value == nil {
		return property.Nil{}
	}

	return document.value
}

/******************************************
 * Introspection Methods
 ******************************************/

// IsBool returns TRUE if the document's value is a boolean.
func (document Document) IsBool() bool {
	return property.IsBool(document.value)
}

// IsInt returns TRUE if the document's value is an integer.
func (document Document) IsInt() bool {
	return property.IsInt(document.value)
}

// IsInt64 returns TRUE if the document's value is a 64-bit integer.
func (document Document) IsInt64() bool {
	return property.IsInt64(document.value)
}

// IsFloat returns TRUE if the document's value is a floating-point number.
func (document Document) IsFloat() bool {
	return property.IsFloat(document.value)
}

// IsNil returns TRUE if the document's value is nil.
func (document Document) IsNil() bool {
	return document.getValue().IsNil()
}

// IsMap returns TRUE if the document's value is a map.
func (document Document) IsMap() bool {
	return property.IsMap(document.value)
}

// IsSlice returns TRUE if the document's value is a slice.
func (document Document) IsSlice() bool {
	return property.IsSlice(document.value)
}

// IsString returns TRUE if the document's value is a string.
func (document Document) IsString() bool {
	return property.IsString(document.value)
}

// NotNil returns TRUE if the document's value is not nil.
func (document Document) NotNil() bool {
	return !document.IsNil()
}

// IsSameOrigin returns TRUE if this document's id shares an origin (scheme, host, and port) with the
// given URL.  It compares the id, not the ActivityStreams "origin" property that Origin returns.
func (document Document) IsSameOrigin(url string) bool {

	// NOTE: An id with no origin, such as a missing id or a urn:uuid, matches nothing, so a caller
	// can never be vouched for by a value that names no host.
	return uri.IsSameOrigin(document.ID(), url)
}

/******************************************
 * Getter Methods
 ******************************************/

// Value returns the generic data stored in this Document
func (document Document) Value() any {
	return document.getValue().Raw()
}

// Clone returns a deep copy of the Document.
func (document Document) Clone() Document {

	return Document{
		client:     document.client,
		httpHeader: document.httpHeader.Clone(),
		value:      document.getValue().Clone(),
		Metadata:   document.Metadata.Clone(),
	}
}

// Get returns a sub-property of the current document
func (document Document) Get(key string) Document {

	// Special handling for string values
	if document.IsString() {

		// Individual values are assumed to be a document ID.
		// So if te ID property was requested, then just return it
		if key == vocab.PropertyID {
			return document
		}

		// All other properties require a Load from the Interweb, but only if the
		// string is actually a loadable URL. Empty or non-URL strings have
		// nothing to load; attempting to do so would recurse infinitely through
		// Load -> ID -> Get (Load and ID both call Get on this same string).
		if uri.IsValidURL(document.rawString()) {
			if loaded, err := document.Load(); err == nil {
				document.value = loaded.value
			}
		}
	}

	// Retrieve the value from the property.Value
	if value := document.getValue().Get(key); !value.IsNil() {
		return document.sub(value)
	}

	// Nil document if the property doesn't exist
	return NilDocument()
}

/******************************************
 * Conversion Methods
 ******************************************/

// Slice returns the document's value as a slice.
func (document Document) Slice() sliceof.Any {
	return convert.SliceOfAny(document.getValue().Raw())
}

// SliceOfDocuments transforms the current object into a slice of separate
// Document objects, one for each value in the current document array.
func (document Document) SliceOfDocuments() sliceof.Object[Document] {
	result := sliceof.NewObject[Document]()

	for item := range document.Range() {
		result = append(result, item)
	}

	return result
}

// SliceOfString transforms the current object into a slice of strings
// by extracting the ID from each sub-document in the current document array.
func (document Document) SliceOfString() sliceof.String {
	result := sliceof.NewString()

	for item := range document.Range() {
		result = append(result, item.ID())
	}

	return result
}

// Bool returns the current object as a boolean value
func (document Document) Bool() bool {

	if getter, ok := document.value.(property.BoolGetter); ok {
		return getter.Bool()
	}

	return convert.Bool(document.getValue().Head().Raw())
}

// Float returns the current object as a floating-point value
func (document Document) Float() float64 {

	if getter, ok := document.value.(property.FloatGetter); ok {
		return getter.Float()
	}

	return convert.Float(document.getValue().Head().Raw())
}

// Int returns the current object as an integer value
func (document Document) Int() int {

	if getter, ok := document.value.(property.IntGetter); ok {
		return getter.Int()
	}

	return convert.Int(document.getValue().Head().Raw())
}

// Load retrieves a JSON-LD document from its remote server
func (document Document) Load(options ...any) (Document, error) {

	const location = "hannibal.streams.document.Load"

	// Get the document ID
	documentID := document.ID()

	// Guarantee that we have an actual value
	if documentID == "" {
		return NilDocument(), nil
	}

	// Require that the DocumentID is a valid URL
	if !uri.IsValidURL(documentID) {
		return NilDocument(), derp.BadRequest(location, "Document ID is not a valid URL", documentID)
	}

	// Try to load the document from the Interwebs
	result, err := document.getClient().Load(documentID, options...)

	if err != nil {
		return result, derp.Wrap(err, location, "Unable to load document by ID", document.Value())
	}

	// Success??
	return result, nil
}

// MustLoad retrieves a JSON-LD document from its remote server. It discards any error, and
// returns whatever Load returned with it.
func (document Document) MustLoad(options ...any) Document {
	result, _ := document.Load(options...)
	return result
}

// LoadLink loads a new JSON-LD document from a link or ID string.
// If the current document has already been loaded (because it's a map)
// then it is returned as-is.
func (document Document) LoadLink(options ...any) Document {

	// If this document is a string, then assume it's
	// an ID and load it from the Intertubes.
	if document.IsString() {
		return document.MustLoad(options...)
	}

	// Nothing to load. We already have a map.
	return document
}

// Map returns the current object as a map. A list uses its first item, and a string becomes a
// map that holds only its id.
func (document Document) Map() mapof.Any {

	// Traverse slices, if necessary
	value := document.getValue().Head()

	// NOTE: This is the Document's own map, not a copy. Changing it changes the Document.
	if getter, ok := value.(property.MapGetter); ok {
		return getter.Map()
	}

	if getter, ok := value.(property.IsStringer); ok {
		return mapof.Any{vocab.PropertyID: getter.String()}
	}

	return mapof.NewAny()
}

// MapKeys returns a slice of all keys in the current map
func (document Document) MapKeys() sliceof.String {

	if mapper, ok := document.value.(property.IsMapper); ok {
		return mapper.MapKeys()
	}

	return []string{}
}

// String returns the current object as a pure string (no HTML).
// This value is filtered by blueMonday, so it is safe to use in HTML.
func (document Document) String() string {
	return plainText(document.rawString())
}

// HTMLString returns the document's value as an HTML string.
// The value is filtered by bluemonday, so it is safe to use in HTML.
func (document Document) HTMLString() string {
	result := document.rawString()
	return ugcPolicy.Sanitize(result)
}

// String returns the current object as a string value
func (document Document) rawString() string {

	if getter, ok := document.value.(property.IsStringer); ok {
		return getter.String()
	}

	return convert.String(document.getValue().Head().Raw())
}

// Time returns the current object as a time value
func (document Document) Time() time.Time {

	if getter, ok := document.value.(property.TimeGetter); ok {
		return getter.Time()
	}

	return convert.Time(document.getValue().Head().Raw())
}

/******************************************
 * Array-based Iterators
 ******************************************/

// Len returns the length of the document.
// If the document is nil, then this method returns 0
// If the document is a slice, then this method returns the length of the slice
// Otherwise, this method returns 1
func (document Document) Len() int {
	return document.getValue().Len()
}

/******************************************
 * List-based Iterators
 ******************************************/

// Head returns the first object in a slice.
// For all other document types, it returns the current document.
func (document Document) Head() Document {
	return document.sub(document.getValue().Head())
}

// Tail returns all records after the first in a slice.
// For all other document types, it returns a nil document.
func (document Document) Tail() Document {
	return document.sub(document.getValue().Tail())
}

// IsEmptyTail return TRUE there are no array items after the current value
func (document Document) IsEmptyTail() bool {
	return document.getValue().Len() < 2
}

// IsEmpty returns TRUE if the documet is empty.
func (document Document) IsEmpty() bool {
	return document.getValue().Len() == 0
}

// NotEmpty returns TRUE if the document has at least one item in it.
func (document Document) NotEmpty() bool {
	return document.getValue().Len() > 0
}

/******************************************
 * Helpers
 ******************************************/

// Client returns the HTTP client used for this document
func (document *Document) Client() Client {
	return document.client
}

// SetValue sets the value of this document to a new value.
func (document *Document) SetValue(value property.Value) {
	document.value = value
}

// WithOptions applies the provided options to the Document in place.
func (document *Document) WithOptions(options ...DocumentOption) {
	for _, option := range options {
		option(document)
	}
}

// AddOptions returns a copy of the Document with the provided options applied.
func (document Document) AddOptions(options ...DocumentOption) Document {
	for _, option := range options {
		option(&document)
	}
	return document
}

// getClient returns the Document's Client. A Document made without NewDocument has none, and loads nothing.
func (document *Document) getClient() Client {

	if document.client != nil {
		return document.client
	}

	return NewNilClient()
}

// sub returns a new Document with a new VALUE, all of the same OPTIONS as this original
func (document *Document) sub(value property.Value) Document {
	return Document{
		value:      value,
		client:     document.client,
		httpHeader: document.httpHeader,
	}
}
