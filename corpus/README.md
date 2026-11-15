# corpus

ActivityStreams documents for tests. This package sits beside hannibal's other packages (see the [main README](../README.md)) and is imported by tests only: [delivery](../delivery/) runs every document through its fan-out and its send, and Emissary publishes the same documents through its outbox.

There are three sets:

- **`Fediverse(world)`** returns 73 activities shaped like what Mastodon, Pleroma, Akkoma, Misskey, Lemmy, PeerTube, Pixelfed, GoToSocial, WriteFreely, WordPress, Funkwhale, BookWyrm, Mobilizon, Friendica and Hubzilla publish, along with activities that carry blind recipients, unusual addressing, and awkward values. Each one records the recipients its addressing should reach, written by hand.
- **`W3C()`** returns the 211 example documents that the W3C publishes with the ActivityStreams 2.0 specifications.
- **`Rejected()`** returns 16 W3C documents that a strict parser should refuse, such as one whose `actor` is a number.

### Using corpus

```go
// Say who sends the examples, and who follows them
world := corpus.World{
    Actor:     "https://example.com/@author",
    Followers: "https://example.com/@author/pub/followers",
    Members:   []string{"https://follower.example.org/users/alice", "mailto:dana@example.org"},
}

examples, err := corpus.Fediverse(world)

for _, example := range examples {

    // Every call decodes a new copy
    document, err := example.Document()

    // ...send the document, then compare who received it with example.Recipients
}
```

## Where the documents come from

The W3C documents are copied unchanged from the `test` folder of [w3c/activitystreams](https://github.com/w3c/activitystreams), under the [W3C Software License](w3c/LICENSE.md). This package includes material copied from the Activity Streams 2.0 test documents. Copyright © 2014-2016 W3C® (MIT, ERCIM, Keio, Beihang). Five of the published files are left out because they do not parse as a JSON object.

The fediverse documents were written for this package from each project's published wire format. They are not captured traffic, and every name, host and id in them is invented.
