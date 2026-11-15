# corpus — Notes for AI Agents

See [README.md](README.md) for what the package holds. These are the rules that the files do not show.

- **The `recipients` of a fediverse example are written by hand, and must stay that way.** They are the answer that the delivery tests check against. Do not generate them by running the code under test, and when a test disagrees with one, work out which of the two is wrong before changing either.
- **`{{actor}}` and `{{followers}}` are the only tokens.** `Fediverse` replaces them in the document and in the recipients. In the recipients, `{{followers}}` becomes the World's members, and an address keeps the place where it first appears.
- **Every host is a subdomain of `example.org`.** `remote` refuses a top-level domain that is not registered, so `mastodon.example` cannot be delivered to, and a send test would fail before it reached the inbox.
- **`blind.example.org` is only ever a blind recipient.** The tests refuse any sent or served body that contains that host, so never use it in `to`, `cc`, or content.
- **A recipient order is part of the answer.** The fan-out reads `to`, `cc`, `bto`, then `bcc`, drops Public, and keeps the first of a duplicate.
- **The W3C files are copied unchanged.** Do not edit one to make a test pass. `w3c/LICENSE.md` must stay with them.
- **An activity with no addressing is delivered to no one.** Many fediverse examples record an empty list for that reason: Mastodon sends a `Like`, a `Follow` and an `Undo` with no `to`, and works out the inbox itself. The delivery pipeline never adds a recipient, so an application must address these before it publishes them.
- **Two lines in `corpus.go` cannot be reached by a test:** the errors from reading and parsing an embedded fediverse file, which the compiler has already put in the binary.
