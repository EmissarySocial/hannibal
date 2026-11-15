// Package corpus holds ActivityStreams documents for tests: a set shaped like what fediverse
// servers publish, and the examples that the W3C publishes with the ActivityStreams
// specifications. Each fediverse example lists the recipients its addressing should reach, so
// a delivery test can check its result against an answer that was written by hand.
//
// The package is imported by tests only. Nothing in hannibal's own code depends on it, and
// the documents are embedded, so a test in another module reads the same files.
package corpus
