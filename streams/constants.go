package streams

// MaxResponseSize is the largest ActivityStreams document, in bytes, that a client reads
const MaxResponseSize = 1 << 20 // Mastodon's own limit, so any document Mastodon can federate fits

// maxLinkedTags is the number of a document's tags that SummaryWithTagLinks searches for. The
// tags come from another server, and each one adds work at every character of the summary.
const maxLinkedTags = 64
