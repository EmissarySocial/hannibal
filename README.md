# 🐘 Hannibal

<img src="https://github.com/benpate/hannibal/raw/main/meta/logo.jpg" style="width:100%; display:block; margin-bottom:20px;"  alt="Oil painting titled: Hannibal in the Alps, by R.B. Davis">

[![Go Reference](https://pkg.go.dev/badge/github.com/benpate/hannibal.svg)](https://pkg.go.dev/github.com/benpate/hannibal)
[![Version](https://img.shields.io/github/v/release/benpate/hannibal?include_prereleases&style=flat-square&color=brightgreen)](https://github.com/benpate/hannibal/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/benpate/hannibal/go.yml?style=flat-square)](https://github.com/benpate/hannibal/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/benpate/hannibal?style=flat-square)](https://goreportcard.com/report/github.com/benpate/hannibal)
[![Codecov](https://img.shields.io/codecov/c/github/benpate/hannibal.svg?style=flat-square)](https://codecov.io/gh/benpate/hannibal)

## Triumphant ActivityPub for Go

Hannibal is an experimental ActivityPub library for Go. Its goal is to be a robust, idiomatic, and thoroughly documented ActivityPub implementation that fits into your application without any magic or drama.

## PLEASE USE.. \*with caution

I would be honored for you to see if Hannibal works for your project. It is being used successfully in several production websites.

However, it _is_ still subject to significant breaking changes. So, please [reach out](https://mastodon.social/@benpate) and I am happy to help you get things working in your own software. And, please be prepared for the ground to shift under your feet with newer versions.

## Packages

Like the ActivityPub spec itself, Hannibal is broken into several layers:

### pub - ActivityPub client/server

https://www.w3.org/TR/activitypub/

This is not an ActivityPub framework, but a simple library that easily plugs into your existing app. Add ActivityPub behaviors to your existing handlers, and send ActivityPub messages to other servers.

### vocab - ActivityStreams Vocabulary

https://www.w3.org/TR/activitystreams-vocabulary/

The `vocab` package includes the standard ActivityStream vocabulary, including names of actions, objects and properties used in ActivityPub.

### streams - ActivityStreams data structures

https://www.w3.org/TR/activitystreams-core/

The `streams` package contains common data structures defined in the ActivityStreams spec, notably definitions for: `Document`, `Collection`, `OrderedCollection`, `CollectionPage`, and `OrderedCollectionPage`. These are used by ActivityPub to send and receive multiple records in one HTTP request.

This package also includes a lightweight wrapper around generic data structures (like `map[string]any` and `[]any`) that makes it easy to access data structures within an ActivityStreams/JSON-LD document.

### metadata - Server-side document metadata

The `metadata` package holds what your server knows *about* a document, such as reply counts, relationships, and the current viewer's moderation labels. None of it is ever part of the document's wire value.

### sigs - HTTP Signatures and Digests

https://datatracker.ietf.org/doc/draft-ietf-httpbis-message-signatures

The `sigs` package creates and verifies HTTP signatures and Digests.

## Image Credit

The banner is *Hannibal in the Alps* by Richard Barrett Davis (1782–1854). The work is in the public domain.

## Pull Requests Welcome

This library is a work in progress, and will benefit from your experience reports, use cases, and contributions. If you have an idea for making this library better, send in a pull request. We're all in this together! 🐘
