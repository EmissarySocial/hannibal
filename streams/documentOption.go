package streams

import (
	"net/http"

	"github.com/benpate/hannibal/metadata"
	"github.com/benpate/remote"
)

// DocumentOption is a function that configures a Document during construction.
type DocumentOption func(*Document)

// WithClient sets the Client that loads remote documents for this Document. A nil Client
// leaves the Document able to load nothing.
func WithClient(client Client) DocumentOption {
	return func(doc *Document) {
		if client == nil {
			doc.client = NewNilClient()
		} else {
			doc.client = client
		}
	}
}

// WithDefaultClient lets the Document load remote documents straight from their servers, with
// no cache, using the provided options on every request
func WithDefaultClient(options ...remote.Option) DocumentOption {
	return func(doc *Document) {
		doc.client = NewDefaultClient(options...)
	}
}

// WithHTTPHeader attaches an HTTP header to the document
func WithHTTPHeader(httpHeader http.Header) DocumentOption {
	return func(doc *Document) {
		doc.httpHeader = httpHeader
	}
}

// WithMetadata attaches metadata to the document, replacing all of it, so it belongs before any
// option that sets a single metadata field.
func WithMetadata(value metadata.Metadata) DocumentOption {
	return func(doc *Document) {
		doc.Metadata = value
	}
}

// WithNoStore marks the document so that no cache ever writes it.  Apply it after WithMetadata.
func WithNoStore() DocumentOption {
	return func(doc *Document) {
		doc.Metadata.NoStore = true
	}
}

// WithLabels attaches the current viewer's moderation labels to the document.  Apply it after WithMetadata.
func WithLabels(labels metadata.LabelSet) DocumentOption {

	// Copy once so that later changes to the caller's labels cannot reach the option, and again
	// for each document, so that no two documents share one set of labels
	labels = labels.Clone()

	return func(doc *Document) {
		doc.Metadata.Labels = labels.Clone()
	}
}

// WithRelation records the document's relation to another, such as a reply, setting the type and
// the related URL together.  Apply it after WithMetadata.
func WithRelation(relationType string, href string) DocumentOption {
	return func(doc *Document) {
		doc.Metadata.RelationType = relationType
		doc.Metadata.RelationHref = href
	}
}

// WithDocumentCategory records the document's high-level category.  Apply it after WithMetadata.
func WithDocumentCategory(category string) DocumentOption {
	return func(doc *Document) {
		doc.Metadata.DocumentCategory = category
	}
}
