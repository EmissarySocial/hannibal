# Hannibal / collection

Collection serves an ActivityStreams [`OrderedCollection`](https://www.w3.org/TR/activitystreams-vocabulary/#dfn-orderedcollection)
(or a single [`OrderedCollectionPage`](https://www.w3.org/TR/activitystreams-vocabulary/#dfn-orderedcollectionpage))
over HTTP. You supply two functions — one that counts the items and one that iterates them — and
`Serve` handles paging, content negotiation, and JSON-LD rendering.

```go
func handleOutbox(writer http.ResponseWriter, request *http.Request) error {

	// Returns the total number of items in the collection
	countFunc := func() (int64, error) {
		return myService.CountActivities(actorID)
	}

	// Returns an iterator over items, starting after the given cursor
	iteratorFunc := func(startIndex string) (iter.Seq[mapof.Any], error) {
		return myService.ActivitiesAfter(actorID, startIndex)
	}

	return collection.Serve(writer, request, collectionID, countFunc, iteratorFunc,
		collection.WithAttributedTo(actorID),
	)
}
```

## Paging

`Serve` inspects the request's `after` query parameter:

- **no `after`** — returns the top-level `OrderedCollection` (with `first`/`last` page links and the total count).
- **`after=<id>`** — returns the `OrderedCollectionPage` of items following that cursor.

Every response is served as `application/activity+json`. `Serve` takes a standard `http.ResponseWriter` and `*http.Request`, so it works under any router; with echo, pass `ctx.Response()` and `ctx.Request()`. The cursor in `first` and `next` is an item's id, query-escaped. A full page whose last item has no id names no `next` page.

The `countFunc` and `iteratorFunc` are expected to already encode any filtering (actor permissions,
visibility, cursors); `Serve` does not add its own access control.

## Options

- `WithAttributedTo(id)` — sets the collection's `attributedTo` property.
- `WithAudience(id)` — sets the collection's `audience` property.
- `WithSSEEndpoint(url)` — advertises a Server-Sent Events endpoint for live updates.
