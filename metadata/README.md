## Hannibal / metadata

This package holds what a server knows *about* an ActivityStreams document, as opposed to what the
document says about itself. Nothing here is ever part of a document's wire value.

`Metadata` has two layers:

- **Document facts** are the same for every viewer and are stored alongside cached documents: a hashed
  ID, the document's category, its relation to another document (a reply, announce, or like), and its
  reply, announce, and like counts.
- **Per-load values** are attached by server code each time a document loads, and are never stored or
  serialized: the current viewer's moderation `Labels`, and the `NoStore` flag that keeps a document
  out of every cache.

Documents in the [streams](../streams/README.md) package carry a `Metadata` value, and the
`streams.With*` document options set it.
