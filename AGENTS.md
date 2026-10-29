# hannibal — Notes for AI Agents

Hannibal is a Go ActivityPub library, layered roughly like the spec itself: [streams](streams/) and [property](property/) wrap JSON-LD data, [vocab](vocab/) holds the vocabulary constants, [sigs](sigs/) signs and verifies HTTP requests, [router](router/) and [validator](validator/) receive inbound activities, [outbox](outbox/) and [sender](sender/) deliver outbound ones, [collection](collection/) serves and [collections](collections/) traverses AS2 collections, and [clients](clients/) provides stackable `streams.Client` implementations. See [README.md](README.md) for the overview; [datetime/AGENTS.md](datetime/AGENTS.md) has its own notes.

## streams.Document shares live data — Map() and Object() are not clones

- **For a map-backed Document, `Map()` returns the LIVE underlying `map[string]any`, not a copy.** `Object().Map()` on an embedded object is the same live map, so mutations write through to every Document sharing that value. A string-reference document instead yields a fresh `{"id": ...}` map, and writes to that map go nowhere — callers that mutate must set the result back onto the parent. Use `Clone()` when you need an independent copy.
- **`Map()` options delete keys from the map they return.** `OptionStripContext` and `OptionStripRecipients` call `delete()` on the result — which for a map-backed document is the shared original. `Clone()` first if the source document must survive intact.
- **`Get()` on an embedded map returns a sub-Document over the same underlying containers.** Mutating through a sub-document writes through to the parent; this write-through behavior is relied on by callers, so don't "fix" it.

## SetProperty silently no-ops unless the value is already a map

`SetProperty` has a value receiver and discards the `property.Value` returned by `Set`. `property.Map.Set` mutates in place so it works; `String.Set` and `Nil.Set` return a NEW value that is thrown away, so setting a property on a string- or nil-backed Document does nothing, with no error. The pointer-receiver mutators (`SetString`, `Append`, `AppendString`) reassign `document.value` and are safe on any shape.

## Property access can silently hit the network

`Document.Get(key)` on a string-valued document treats the string as a document ID and LOADS it over HTTP through the injected `streams.Client` for every key except `id`. Innocent-looking accessor chains like `activity.Object().AttributedTo().Name()` can therefore perform remote fetches. `RangeInReplyTo` loads the parent document inside the iterator, and the `DeletedObject` validator issues a GET during inbound validation. Always inject a caching client (`streams.WithClient`) where repeated access is possible; stacked clients must have `SetRootClient` wired so recursive loads re-enter the top of the stack. Predicates load too: `IsActivity`, `IsObject`, `HasContent`, and their siblings call `Type()` or `Content()`. The ones that read only the value in hand are `IsNil`, `IsMap`, `IsString`, and `IsEmpty`. A string is loaded only when `uri.IsValidURL` accepts it, and it rejects `https://remote.example/…` while accepting `https://example.com/…`, so a test that counts hidden loads must use a host it accepts, or its count can never fail.

## A client wrapper that re-loads a different URL MUST spread its options

Any `streams.Client` whose `Load(id string, options ...any)` turns around and loads a *different* URL has to write `innerClient.Load(otherURL, options...)`. Passing `options` without the spread hands the whole slice down as a single `any`, so a layer below that reads them, such as Emissary's `ascache.NewLoadConfig(options...)`, sees one `[]any` element instead of the caller's actual options — and silently drops them. It compiles, and it only misbehaves when an option needed to reach a layer below a wrapper that re-resolves the URL.

This cost hours once. Inbox signature verification failed with `crypto/rsa: verification error` against a rotated key, because Emissary's key finder loaded the fragmented key id (`…#main-key`) with Emissary's `ascache.WithWriteOnly()` to bypass the cache, and the fragment-resolving wrapper called `Load(baseURL, options)` with no spread — so the cache layer never saw `WithWriteOnly`, served the stale key, and nothing anywhere reported a problem. That wrapper, `clients.HashLookup`, is the one to watch, because key lookups are exactly what load a fragmented URL. When reviewing any wrapper in this family, grep for `.Load([^)]*options)` without the `...`; that pattern is almost always a bug.

## Carpool riders get copies, and options load alone

- **Every caller that shares a Carpool Load gets its own `Clone()`, rebound to its own root client.** A `streams.Document` carries the client stack that produced it, so handing one document to every rider would make follow-up loads sign as the first caller, and would share live maps between goroutines (see the `Map()` rule above).
- **The grouping key is the signer plus the URL.** A remote server can refuse one signer and answer another, so a Load signed as Alice must never be handed to Bob, even though the cache below may later serve Alice's copy to Bob. Pass the identity the stack actually signs as.
- **A Load with any options never joins another.** Options are opaque `any` values, so Emissary's `ascache.WithWriteOnly` joining a plain read would receive a cached copy, which is the stale-key failure described above. Keep the bypass when adding options.
- **A Carpool is shared by every stack in a process, and covers only that process.** Build one at startup and pass it in. Several servers each have their own, so it reduces duplicate work but cannot make concurrent writes safe.

## OptionsClient carries options into the loads a document makes on its own

`streams.NewOptionsClient(inner, options...)` is how options reach hidden loads. Bind a document to it with `WithClient`, and every getter that loads a bare-URL value, and every document those loads return, carries the options, because `Load` rebinds each result to the client. Binding decides which loads use it; `SetRootClient` does not. Four rules keep it safe:

- **The constructor never calls `inner.SetRootClient`.** Every stack layer's `New` does, but a binding usually wraps the top of a stack that is already serving other loads, and re-rooting it would send all of them through one document's options.
- **`SetRootClient` passes the call on, like every other layer.** Used as a layer inside a stack, an `OptionsClient` that swallowed it would leave the layers below with an older root, and documents they return would load around the cache, the rules, and the Carpool.
- **Never build a new layer on top of a document's binding.** That layer's constructor would call `SetRootClient` on the binding, which would pass it into the live stack.
- **Options are combined with `slices.Concat`, never `append`.** Every document bound to one client shares its slice, and an append writes into spare capacity that concurrent loads also use. `TestOptionsClient_ConcurrentLoadsDoNotShareOptions` catches it under `-race`.

Anything bound through it skips the Carpool: its options make every load carry options, and a Load with options loads alone (above). Emissary's normalizer relies on that to keep nested loads from waiting on their own leader.

## Metadata.Labels and NoStore are internal, and only server code may set them

`metadata.Metadata.Labels` is the current viewer's moderation verdict. Its single `bson:"-" json:"-"` tag keeps everything inside it out of shared caches and off the wire, so a field added to `Label` later is covered without another tag, and a remote server cannot spoof a label because the JSON parser fills only a document's value, never its Metadata.

`metadata.Metadata.NoStore` tells every cache never to write the document. It is set by code, never by data: like `Labels`, it carries `bson:"-" json:"-"`, so no stored copy and no remote document can set it, and `TestDocument_UnmarshalJSON_CannotSetNoStore` pins that. Never serialize it or read it from a document's value, because a remote that could set it could keep its own documents out of every cache, which defeats cooldowns that need a cached copy. It is unrelated to the `Cache-Control` header. Set it with `streams.WithNoStore()`, and apply that after any `WithMetadata`: `WithMetadata` replaces all of a document's metadata, so it silently drops `NoStore`, `WithLabels`, `WithRelation`, or `WithDocumentCategory` applied before it (`TestWithMetadata_ReplacesFieldOptions` pins the order).

## Reading text: String() and HTMLString() both sanitize

`Document.String()` runs bluemonday `StrictPolicy` (strips ALL HTML) then unescapes entities; `HTMLString()` runs `UGCPolicy`. Both policies are built once and shared by every Document ([sanitize.go](streams/sanitize.go)), because building one per call cost up to 2,000 allocations per accessor; never call `AllowAttrs` or any other mutator on them — a caller that needs different rules builds its own policy. The unsanitized string is only reachable via the unexported `rawString` or the raw `Value()`. Federated content must go through one of the sanitizing accessors — never add an exported raw-string accessor.

## `URL()` reads a list of Links the way Mastodon does

`Document.URL()` returns the `href` of the `text/html` Link when `url` is a list, even when a plain string comes first; otherwise the first value, as a string or a Link's `href`. PeerTube channels and Bandwagon artists list an RSS Link beside the profile page, and before 2026-10-05 `URL()` returned `""` for any list of Links, so every Emissary reader lost the profile link (FUNKWHALE D13). `TestDocument_URL_Links` pins each shape.

## InteractionPolicy reads only GoToSocial's vocabulary

[interactionPolicy.go](streams/interactionPolicy.go) reads GoToSocial's `interactionPolicy` and nothing else. PeerTube's `commentsEnabled`, Mobilizon's `repliesModerationOption`, Pixelfed's `capabilities`, FEP-5624's `canReply`, and Hubzilla's `commentPolicy` were read briefly on 2026-10-22 and removed by Ben's decision the same day; do not add them back without asking.

`CanInteract` applies GoToSocial's rules, in this order, and the first that matches decides:

1. An anonymous actor is denied.
2. The author may always interact with their own object. Under FEP-044f, a self-quote is the only quote that needs no approval.
3. An actor the object mentions may always reply. GoToSocial also exempts the actor being replied to; callers do not pass that actor, because finding it costs a fetch (Ben, 2026-10-22).
4. The most specific entry wins: the actor's own IRI, then a collection, then Public. At the same level, automatic approval wins over manual.
5. Anything else is denied.

GoToSocial's exception for an object that is itself pending approval is not applied, because a policy cannot know whether its object is pending. These rules are easy to get wrong:

- **A missing permission is not "nobody", but an empty list is.** GoToSocial approves Public for any sub-policy a post does not define, so a missing or `null` list means the default. `{"automaticApproval": []}` is defined, and admits only the author and mentions (Ben, 2026-10-22). JSON-LD canonicalization would read `[]` as missing, which is why Emissary never publishes one.
- **A missing `canQuote` approves only the author,** because FEP-044f requires approval for every quote but a self-quote. Quote posts are not built (emissary-specs BUG-286); revisit this default then.
- **The most specific entry wins, not the most permissive.** A post open to Public with one actor under `manualApproval` holds that actor's interactions.
- **A policy is never loaded from an IRI.** On 2026-10-22, every implementation checked (GoToSocial, Mastodon, Pixelfed, PieFed, Friendica, Iceshrimp.NET, Fedify) embedded the policy and each sub-policy inline, and none dereferenced an IRI there: GoToSocial ignores one, and Mastodon raises a `TypeError`. An IRI is an empty policy with its `id` set, so loading one would only add a remote fetch that any post's author can trigger on every render. Ben chose this, and to keep `InteractionPolicy` a `mapof.Any` wrapper rather than a `streams.Document`.
- **List entries read `id`, then `@id`.** GoToSocial drops object entries, but JSON-LD processors (Fedify, Friendica) read them, because the context types these terms `"@type": "@id"`. Link objects (`href`) are not read: no server sends them (Ben, 2026-10-22).
- **Coercion must accept named slice and map types.** A policy read back from MongoDB arrives as `primitive.A` and `primitive.M`, which a type switch on `[]any` or `map[string]any` does not match, so before 2026-10-22 every cached policy read as an empty list and refused every reply. `reflectList` and `reflectObject` catch any slice, array, or string-keyed map; `TestInteractionPolicy_NamedTypes` pins it. Found by Emissary's `integration-tests/replycontrols/run.sh`, not by any unit test, because they all decode from JSON.
- **`Map()` is read-only.** It may return the very map the policy was created from, as `Image` does; copying it was declined (Ben, 2026-10-22).
- **`IsMember` is called only for IRIs that might be collections**, never for Public, the actor, or the author, because each call costs the caller a database lookup. `TestInteractionPolicy_CanInteract_LooksUpOnlyWhatItNeeds` and `TestInteractionRequest_IsMemberOfAny` pin it.

## Inbound requests fail closed, with deliberate status codes

- **Malformed JSON in `router.ReceiveRequest` returns 400, not 500.** The `derp.WithBadRequest()` option on that `json.Unmarshal` wrap is important: unmarshal errors are codeless and would otherwise default to 500 for every junk POST from a crawler. `MaxBodySize` caps the body and errors rather than truncating.
- **The validator chain fails closed.** `ResultInvalid` rejects, `ResultValid` accepts, `ResultUnknown` continues to the next validator — and if every validator returns Unknown, validation FAILS. An unsigned request gets `ResultUnknown` from `HTTPSig`, so unsigned inbound activities are rejected unless another validator vouches for them. Validation failure surfaces as 401 Unauthorized.
- **`HTTPSig` enforces two identity checks:** the verified signature's actor must equal the activity's `actor`, and the default key finder only accepts a key whose `id` the signing actor actually publishes. Both checks prevent signing with someone else's key; don't weaken either.

## RangeInReplyTo yields the parent's attributedTo first

`RangeAddressees` reads `actor`/`to`/`cc`/`bto`/`bcc`/mentions but NOT `attributedTo`, so without the explicit author yield at the top of `RangeInReplyTo` a reply to a Note (whose author lives in `attributedTo`) would never reach that author. The author-then-addressees order is deliberate; don't simplify it down to `RangeAddressees` alone.

## collections traversal guards — all are important

A collection's `next` chain and `items` arrays are remote-controlled data, so three guards bound a traversal and none subsumes another. In `RangePages`: the empty-page flag catches WriteFreely-style loops of EMPTY pages, and the page cap (`defaultMaxPages`, override via `WithMaxPages`) catches cycles of NON-empty pages the flag cannot see. In `RangeDocuments`: the document cap (`defaultMaxDocuments`, override via `WithMaxDocuments`) bounds total yields, because the page cap alone still admits a server stuffing each page with an enormous `items` array. Wrappers over `RangePages` (e.g. `RangeDocuments`) must forward `options...` — a missing spread silently drops a caller's caps.

## SSRF guard: AllowPrivateIPs is a per-instance opt-in, never a global

Both delivery paths inherit `remote`'s default refusal to connect to private/loopback addresses. The opt-outs are per-instance functional options — `outbox.WithAllowPrivateIPs(bool)` on the Actor and `sender.AllowPrivateIPs(bool)` on the Sender — and default FALSE. Do not reintroduce a package-level flag; tests that deliver to `httptest` loopback servers pass the option instead. The `DeletedObject` validator's outbound GET (to a URL taken from an untrusted inbound activity) deliberately keeps the guard active. Its unexported `allowPrivateIPs` field exists only so this package's tests can reach a loopback server; there is no exported way to set it, and `TestDeletedObject_GuardStaysOn` pins that `NewDeletedObject` keeps the guard.

## Every ActivityStreams read is capped at `streams.MaxResponseSize`

`remote` reads the whole body of every response, decoded or not, up to 1 GB by default, so one untrusted server can spend that much memory on a single request. hannibal's two ActivityStreams reads, `DefaultClient.Load` and the `DeletedObject` validator, cap the body at `streams.MaxResponseSize`, 1 MB. The delivery POSTs in `outbox` and `sender` set no cap, so their responses are still read under remote's 1 GB default. That is Mastodon's own fetch limit (`body_with_limit` in Mastodon's `app/lib/request.rb`), so no document Mastodon can federate is refused. The cap counts bytes rather than checking a document type, because the type is unknown until the body has been read and parsed. A caller's `remote.Option` runs at `Send`, after the cap is set, so a caller that knows a document is larger can raise it. sherlock's `activitypub.Client` reads the same constant, and any new `remote.Get` of an ActivityStreams document here must set it as well.

## outbox and sender are two outbound paths with different semantics

- **`outbox.Actor` delivers inline and synchronously.** `Send` filters out empty, `as:Public`, and self recipients, and swallows per-recipient failures via `derp.Report` (fire-and-forget). `SendOne` bypasses those filters — an external delivery loop built directly on `SendOne` must re-implement them or it will attempt deliveries to the Public URI.
- **`sender.Sender` only ENQUEUES; the turbine `Consumer` does the HTTP.** `Send` and `SendToAllRecipients` publish queue tasks — without a connected consumer nothing is ever delivered. Retry policy lives in `SendToSingleRecipient`: HTTP 429 requeues after the Retry-After interval, other 4xx is a permanent `Failure` (no retry), everything else is a retryable `Error`.
- **`SendToAllRecipients` strips `bto`/`bcc` before fan-out** (per the AP spec); `outbox` builders like `SendCreate` embed `document.Map()` — the live map — into the wire message.
- **A single-recipient task carries its payload as `body`, a JSON string serialized once per fan-out.** `SendToSingleRecipient` POSTs `body` byte-for-byte, so the signature's digest covers exactly what was queued. Tasks queued by older versions carry an `activity` map instead, and that path serializes it as before. **Never remove the `activity` fallback:** a task can sit in a stored queue for hours of retries, or indefinitely on a server that was down, and removing the fallback would make those tasks fail. A task with neither is a permanent `Failure`, not a POST of `{}`.
- **An older process must never read a `body`-only task.** Its handler would miss `activity`, POST `{}`, get a 4xx, and drop the delivery for good. Upgrade every process sharing a queue at once, and do not roll back below this version while delivery tasks are still queued.

## Timestamp formats: AS2 in datetime, HTTP Date in sigs

AS2 properties use RFC3339 via the [datetime](datetime/) package; the IMF-fixdate formatter for the HTTP `Date` header is unexported inside [sigs](sigs/) ON PURPOSE, so document-building code can never grab the wrong one. Don't export it or add an HTTP date helper to datetime. Details in [datetime/AGENTS.md](datetime/AGENTS.md).

## property package coercion rules worth knowing

- **Every scalar behaves as a one-element list:** `Head()` returns itself, `Tail()` returns `Nil`, `Len()` is 1. `Map.Len()` is also 1 — a map is one entity, not a count of its keys.
- **A bare string is treated as an object reference:** `String.Get` answers only `id` (returning itself); setting any property on a String promotes it to `Map{"id": old}`.
- **Empty means nil:** `String("")` and a zero-length `Map` report `IsNil()`, and `Document.Get` returns `NilDocument` for nil values — so a property stored as `""` reads back as missing.

## `ContextTypeToot` must use Mastodon's `schema` prefix, `http://schema.org#`

The canonical schema.org IRI (`https://schema.org/`) looks like the correct value and is not. Software that expands JSON-LD looks profile-field values up by their expanded IRI, so `value` must expand to exactly what Mastodon publishes. Funkwhale accepts only `http://schema.org#value` and the Litepub equivalent, so any actor with a `PropertyValue` attachment failed its validation (`attachment.value: This field is required`) and could not be followed. `a5c1589` made that change in May 2026, and it shipped from v0.17.7 until it was reverted.

Plain-JSON readers (Mastodon, Misskey, Pleroma, Pixelfed, Lemmy, PeerTube, WriteFreely, WordPress, NodeBB) match the literal `PropertyValue` and `value` keys and never read the prefix, so the change is invisible to them. Funkwhale is the case that catches a regression: expand an actor with pyld 2.0.4 and check that its attachment carries `http://schema.org#value`.
