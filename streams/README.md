## Hannibal / streams

This package implements the [ActivityStreams 2.0](https://www.w3.org/TR/activitystreams-core/) 
specifications in Hannibal.  Specifically, it provides an overly simplistic view of JSON-LD 
documents, contexts, and the various types of ActivityStream collections.

This is not a rigorous implementation of JSON-LD.  Instead, it is an easy way to navigate 
well-formatted JSON-LD documents and to iterate over their contents, as well as loading 
additional documents from the web when necessary.

```go
// Load a document directly from its URL
document, err := streams.Load("https://your.activitypub.server/@documentId")

document.ID() // returns a string value
document.Content() // returns the content property
document.Published() // returns a time value

// AttributedTo could be many things.. a single value, a link, or
// an array of values and links. Let's make all of that easier.
authors := document.AttributedTo() 

// You could just read the first author directly
authors.Name() // returns the 'Name' string
authors.ID() // returns the 'ID' string

// Or you can use it as an iterator
for author := range document.AttributedTo().Range() {
	author.Value() // returns the whole value from the array
}
```

### Reading links and activities

A few accessors smooth over the different shapes that real servers send.

```go
// URL returns the profile page, even when "url" is a list of Links.
// The text/html Link wins, the way Mastodon reads it.
document.URL()

// UnwrapActivity returns the object inside an activity, looking up to two
// activities deep, so an Announce > Create > Page from Lemmy yields the Page
object := activity.UnwrapActivity()

// IsSameOrigin compares the document's id with another URL by scheme,
// host, and port.  An id with no origin (like a urn:uuid) matches nothing.
if document.IsSameOrigin(actor.ID()) {
	// ...
}
```

### Clients and options

A Document loads linked documents through its `Client` whenever you read a property that holds only a
URL. The [clients](../clients/README.md) package provides layers you can stack on top of the HTTP
client. `NewOptionsClient` wraps any client so that every document it returns, and every document
loaded from those, carries the same set of load options.

```go
// Bind a document to a client that adds options to every load it makes
client := streams.NewOptionsClient(myClient, myOption)
document = document.AddOptions(streams.WithClient(client))
```

### Document metadata

Server code can attach [metadata](../metadata/README.md) to a document as it loads, using
`DocumentOption` functions. None of these values travel over the wire.

```go
document = document.AddOptions(
	streams.WithMetadata(cachedMetadata),          // replaces all metadata, so it goes first
	streams.WithLabels(viewerLabels),              // the viewer's moderation labels
	streams.WithRelation(vocab.RelationTypeReply, parentURL),
	streams.WithDocumentCategory(vocab.ObjectTypeNote),
	streams.WithNoStore(),                         // no cache may write this document
)
```
