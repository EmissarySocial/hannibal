package streams

// MaxResponseSize is the largest ActivityStreams document, in bytes, that a client reads
const MaxResponseSize = 1 << 20 // Mastodon's own limit, so any document Mastodon can federate fits
