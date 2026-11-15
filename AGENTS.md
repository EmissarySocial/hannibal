# hannibal — Notes for AI Agents

Hannibal is a Go ActivityPub library, layered roughly like the spec itself: [streams](streams/) and [property](property/) wrap JSON-LD data, [vocab](vocab/) holds the vocabulary constants, [sigs](sigs/) signs and verifies HTTP requests, [router](router/) and [validator](validator/) receive inbound activities, [outbox](outbox/), [sender](sender/) and [delivery](delivery/) deliver outbound ones, [collection](collection/) serves and [collections](collections/) traverses AS2 collections, and [clients](clients/) provides stackable `streams.Client` implementations. [corpus](corpus/) holds example documents for tests, and has [its own notes](corpus/AGENTS.md). See [README.md](README.md) for the overview; [datetime/AGENTS.md](datetime/AGENTS.md) has its own notes.

## streams.Document shares live data: `Map()` and `Object()` are not clones

- **For a map-backed Document, `Map()` returns the LIVE underlying `map[string]any`, not a copy.** `Object().Map()` on an embedded object is the same live map, so a change to it reaches every Document that shares the value. A string-reference document instead yields a new `{"id": ...}` map, and writes to that map go nowhere: callers that change it must set the result back onto the parent. Use `Clone()` when you need an independent copy. For two days in 2026-11 `Map()` returned a deep copy; that was undone when blind recipients moved to a JSON round trip (Ben, 2026-11-14).
- **`Map()` takes no options.** `OptionStripContext` and `OptionStripRecipients` are removed: they deleted from the live map, and no caller had passed either since 2025-07 (Ben, 2026-11-13).
- **`Get()` on an embedded map returns a sub-Document over the same underlying containers.** Mutating through a sub-document writes through to the parent; this write-through behavior is relied on by callers, so don't "fix" it.

## Every setter needs a variable, because setting can replace the value

`SetProperty`, `SetString`, `SetID`, `SetTo` and the `Append` methods all have pointer receivers, and each one assigns what `property.Value.Set` returns. `Map.Set` changes the map in place, and `String.Set` and `Nil.Set` return a new `Map`, so a setter on a string or a nil Document turns it into an object that holds the property. `document.Object().SetID(x)` therefore does not compile: assign the sub-document to a variable first. On a list, a setter changes the first item and keeps the others. A `Document` made without `NewDocument`, such as a struct field that the JSON left out, reads as nil: every accessor goes through `getValue`, and none may read `document.value` directly.

## Property access can silently hit the network

`Document.Get(key)` on a string-valued document treats the string as a document ID and LOADS it over HTTP through the injected `streams.Client` for every key except `id`. Innocent-looking accessor chains like `activity.Object().AttributedTo().Name()` can therefore perform remote fetches. `RangeInReplyTo` loads the parent document inside the iterator, and the `DeletedObject` validator issues a GET during inbound validation. Always inject a caching client (`streams.WithClient`) where repeated access is possible; stacked clients must have `SetRootClient` wired so recursive loads re-enter the top of the stack. Predicates load too: `IsActivity`, `IsObject`, `HasContent`, and their siblings call `Type()` or `Content()`. The ones that read only the value in hand are `IsNil`, `IsMap`, `IsString`, and `IsEmpty`. A string is loaded only when `uri.IsValidURL` accepts it, and it rejects `https://remote.example/…` while accepting `https://example.com/…`, so a test that counts hidden loads must use a host it accepts, or its count can never fail.

## A Document made without a client loads nothing

`NewDocument`, `NilDocument`, `WithClient(nil)`, a zero `Document`, and one filled by `json.Unmarshal` all carry `NilClient`, whose `Load` makes no request and returns an error. So a Document built from local data, such as an activity about to be published, cannot reach the network on its own. A caller that wants to load passes `WithClient`, or `WithDefaultClient` for the uncached client that goes straight to the origin server. The default key finder in `validator` is the one place in hannibal that asks for it.

- **A hidden load fails silently.** `Get` on a bare URL discards the error from `Load`, so a property of an unloaded URL reads as empty, and `Type()` reads as `Unknown`. Only an explicit `Load` returns the error. `NilClient.Load` writes a debug log line that names the URL, and that line is the only trace of a Document that needed a client and had none.
- **`Actor()` loads, and `ActorID()` does not.** `Actor()` returns nil for a bare URL with no client, so `activity.Actor().ID()` is empty where `activity.ActorID()` is the URL.
- **`IDs()` and `RangeIDs()` read references without loading, and skip one with no id.** They are the plural of `ID()`: a URL yields itself, an object yields its `id` or `@id`, and a list yields each. Use them on any property that names actors or objects, such as `to`, `attributedTo` or `inReplyTo`. `Recipients()` is built on them, so a recipient written as an object is read by its id.
- **`ParseDocument` is the way to turn JSON into a Document.** It takes the options of `NewDocument`, accepts any JSON value, and returns an error with no status, because only the caller knows whether the JSON came from a request or from another server's response. A caller that needs an object asks `IsMap()`.
- **`RangeInReplyTo` yields no one without a client,** and reports the failed load through `derp.Report`. An application that delivers a reply to the parent's author must bind the Document to a client first.
- **A Document handed to a template needs a client when the template reads through a link.** A cached value turned into a Document has none, so an expression such as `$document.AttributedTo.Icon.Href` renders empty until the application binds one with `AddOptions(WithClient(…))`.

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

- **The constructor never calls `inner.SetRootClient`.** Emissary's stack layers call it from their constructors (`NewHashLookup` and `Carpool.Client` in this repository do not), but a binding usually wraps the top of a stack that is already serving other loads, and re-rooting it would send all of them through one document's options.
- **`SetRootClient` passes the call on, like every other layer.** Used as a layer inside a stack, an `OptionsClient` that swallowed it would leave the layers below with an older root, and documents they return would load around the cache, the rules, and the Carpool.
- **Never build a new layer on top of a document's binding.** That layer's constructor would call `SetRootClient` on the binding, which would pass it into the live stack.
- **Options are combined with `slices.Concat`, never `append`.** Every document bound to one client shares its slice, and an append writes into spare capacity that concurrent loads also use. `TestOptionsClient_ConcurrentLoadsDoNotShareOptions` catches it under `-race`.

Anything bound through it skips the Carpool: its options make every load carry options, and a Load with options loads alone (above). Emissary's normalizer relies on that to keep nested loads from waiting on their own leader.

## Metadata.Labels and NoStore are internal, and only server code may set them

`metadata.Metadata.Labels` is the current viewer's moderation verdict. Its single `bson:"-" json:"-"` tag keeps everything inside it out of shared caches and off the wire, so a field added to `Label` later is covered without another tag, and a remote server cannot spoof a label because the JSON parser fills only a document's value, never its Metadata.

`metadata.Metadata.NoStore` tells every cache never to write the document. It is set by code, never by data: like `Labels`, it carries `bson:"-" json:"-"`, so no stored copy and no remote document can set it, and `TestDocument_UnmarshalJSON_CannotSetNoStore` pins that. Never serialize it or read it from a document's value, because a remote that could set it could keep its own documents out of every cache, which defeats cooldowns that need a cached copy. It is unrelated to the `Cache-Control` header. Set it with `streams.WithNoStore()`, and apply that after any `WithMetadata`: `WithMetadata` replaces all of a document's metadata, so it silently drops `NoStore`, `WithLabels`, `WithRelation`, or `WithDocumentCategory` applied before it (`TestWithMetadata_ReplacesFieldOptions` pins the order).

## Reading text: String() and HTMLString() both sanitize

`Document.String()` runs bluemonday `StrictPolicy` (strips ALL HTML) then unescapes entities; `HTMLString()` runs `UGCPolicy`. Both policies are built once and shared by every Document ([sanitize.go](streams/sanitize.go)), because building one per call cost up to 2,000 allocations per accessor; never call `AllowAttrs` or any other mutator on them — a caller that needs different rules builds its own policy. The unsanitized string is only reachable via the unexported `rawString` or the raw `Value()`. Federated content must go through one of the sanitizing accessors — never add an exported raw-string accessor.

## `SummaryWithTagLinks` returns HTML, and walks the summary once

`Summary()` is plain text with its entities already decoded, so `SummaryWithTagLinks` escapes it and adds links, and its result is HTML. Three rules keep it safe, and each replaced a defect a remote actor could trigger (emissary-specs HANNIBAL-CLEANUP §4.1):

- **It walks the summary once and never searches text it has written.** The earlier version searched again after each replacement, and looped forever on a summary that named a tag twice.
- **Letter case is compared one character at a time.** Lower-casing the whole string changes its byte length for some letters (the Kelvin sign is three bytes and folds to a one-byte `k`), so positions found in a lowered copy do not fit the original.
- **Only an absolute `http` or `https` address becomes a link,** and the address and the tag's name are both escaped. Only the first `maxLinkedTags` tags are searched for.

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
- **A signature whose key cannot be found is `ResultUnknown`, and a signature that was checked and did not match is `ResultInvalid`.** A deleted account signs its last `Delete` with a key that is already gone, so `HTTPSig` must leave that request to `DeletedObject`. Returning `ResultInvalid` there stops the chain and refuses every account deletion.
- **`DeletedObject` believes the object's own server, and nobody else.** An unsigned `Delete` is valid when a fetch of the object's URL answers 404, 410, or a document whose type is `Tombstone`. It does not compare the activity's actor with the object, so a followers-only post, which answers 404 to an anonymous fetch, reads as deleted to anyone who knows its URL. The application decides what a valid `Delete` may remove. A 200 answer that is not a JSON object counts as a document that still exists.
- **The verifier never reads `X-Forwarded-Host`.** Any client can send that header, so reading it lets a request signed for one host verify on another. The signed `host` line is checked against the request's own `Host`, or against the name passed in `sigs.WithHostname(hostname)`. A server behind a proxy that rewrites `Host` passes its public name, and decides for itself which header to believe. Emissary passes the answer of its server factory's `Hostname(request)`, which reads the header only when `trustForwardedHost` is set.
- **A body with no `Digest` header is refused, whatever the method.** `Verify` drops the digest from the required fields of a GET, so without this rule a signature made over a bodiless GET would carry any body.
- **A `Date` more than the timeout in the future is refused, and so is a missing `Date` when the verifier requires it signed.** The window is the same 12 hours in both directions.
- **`(request-target)` is built from `URL.EscapedPath()`,** so a signature for `/a%2Fb` does not verify on `/a/b`. The signer and the verifier share `getPathAndQuery`, and they must keep sharing it.
- **`router.WithMaxBodySize` reaches the Digest check.** `NewReceiveConfig` finishes by giving every `HTTPSig` validator the body limit and the key finder from `WithPublicKeyFinder`, after every option has run, so neither depends on its place in the option list and neither discards a validator's own `VerifierOption`s. `sigs.ApplyDigest`, on the signing side, still stops at `re.DefaultMaximum`.
- **A request that cannot be signed is never sent.** `sigs.WithSigner` is the one signing option, and `delivery`, `sender` and `outbox` all call it. It signs in `ModifyRequest`, and returns the signing error with a 400 code so that a caller does not retry. `remote` stops the request there and runs no `AfterRequest` or `OnError` hook, so a circuit breaker never counts the refusal against the host. Each request is signed once.

## RangeInReplyTo yields the parent's attributedTo first

`RangeAddressees` reads `actor`/`to`/`cc`/`bto`/`bcc`/mentions but NOT `attributedTo`, so without the explicit author yield at the top of `RangeInReplyTo` a reply to a Note (whose author lives in `attributedTo`) would never reach that author. The author-then-addressees order is deliberate; don't simplify it down to `RangeAddressees` alone.

## collections traversal guards — all are important

A collection's `next` chain and `items` arrays are remote-controlled data, so three guards bound a traversal and none subsumes another. In `RangePages`: the empty-page flag catches WriteFreely-style loops of EMPTY pages, and the page cap (`defaultMaxPages`, override via `WithMaxPages`) catches cycles of NON-empty pages the flag cannot see. In `RangeDocuments`: the document cap (`defaultMaxDocuments`, override via `WithMaxDocuments`) bounds total yields, because the page cap alone still admits a server stuffing each page with an enormous `items` array. Wrappers over `RangePages` (e.g. `RangeDocuments`) must forward `options...` — a missing spread silently drops a caller's caps.

## SSRF guard: AllowPrivateIPs is a per-instance opt-in, never a global

Both delivery paths inherit `remote`'s default refusal to connect to private/loopback addresses. The opt-outs are per-instance functional options — `outbox.WithAllowPrivateIPs(bool)` on the Actor and `sender.AllowPrivateIPs(bool)` on the Sender — and default FALSE. Do not reintroduce a package-level flag; tests that deliver to `httptest` loopback servers pass the option instead. The `DeletedObject` validator's outbound GET (to a URL taken from an untrusted inbound activity) deliberately keeps the guard active. Its unexported `allowPrivateIPs` field exists only so this package's tests can reach a loopback server; there is no exported way to set it, and `TestDeletedObject_GuardStaysOn` pins that `NewDeletedObject` keeps the guard.

## Every ActivityStreams read is capped at `streams.MaxResponseSize`

`remote` reads the whole body of every response, decoded or not, up to 1 GB by default, so one untrusted server can spend that much memory on a single request. hannibal's two ActivityStreams reads, `DefaultClient.Load` and the `DeletedObject` validator, cap the body at `streams.MaxResponseSize`, 1 MB. The delivery POSTs in `outbox` and `sender` set no cap, so their responses are still read under remote's 1 GB default. That is Mastodon's own fetch limit (`body_with_limit` in Mastodon's `app/lib/request.rb`), so no document Mastodon can federate is refused. The cap counts bytes rather than checking a document type, because the type is unknown until the body has been read and parsed. A caller's `remote.Option` runs at `Send`, after the cap is set, so a caller that knows a document is larger can raise it. sherlock's `activitypub.Client` reads the same constant, and any new `remote.Get` of an ActivityStreams document here must set it as well.

## outbox and sender are two outbound paths with different semantics

- **`outbox.Actor` delivers inline and synchronously.** `Send` filters out empty and self recipients and all three spellings of Public, sends one copy to a recipient named more than once, and swallows per-recipient failures via `derp.Report` (fire-and-forget). `SendOne` bypasses those filters — an external delivery loop built directly on `SendOne` must re-implement them or it will attempt deliveries to the Public URI. A recipient with no inbox, such as a collection, is refused with an error before any request is built.
- **`sender.Sender` only ENQUEUES; the turbine `Consumer` does the HTTP.** `Send` and `SendToAllRecipients` publish queue tasks — without a connected consumer nothing is ever delivered. Retry policy lives in `SendToSingleRecipient`: HTTP 429 requeues after the Retry-After interval, other 4xx is a permanent `Failure` (no retry), everything else is a retryable `Error`.
- **Both paths strip `bto`/`bcc` at every level, from a copy.** Each calls `delivery.StripBlindRecipients`. `SendToAllRecipients` strips before fan-out, and `outbox` strips in `Send` and again in `SendOne`, because Emissary calls `SendOne` directly. The `outbox` builders like `SendCreate` embed `document.Map()`, the live map, into the message, and the strip copies it before anything is removed, so the stored document keeps its blind recipients.
- **`sender` errors name an activity by its `id` and `actor`, through `activityName`.** The activity itself still holds `bto` and `bcc` when those errors are made, and an error detail is written to the log whole.
- **A sending actor that cannot be loaded ends the task only when the error is a 404 or a 410.** Any other error is retried, in `sender` and in `delivery`, so the application's lookup must answer 404 for an actor that does not exist. Emissary's `SendLocator.Actor` does.
- **A single-recipient task carries its payload as `body`, a JSON string serialized once per fan-out.** `SendToSingleRecipient` POSTs `body` byte-for-byte, so the signature's digest covers exactly what was queued. Tasks queued by older versions carry an `activity` map instead, and that path serializes it as before. **Never remove the `activity` fallback:** a task can sit in a stored queue for hours of retries, or indefinitely on a server that was down, and removing the fallback would make those tasks fail. A task with neither is a permanent `Failure`, not a POST of `{}`.
- **An older process must never read a `body`-only task.** Its handler would miss `activity`, POST `{}`, get a 4xx, and drop the delivery for good. Upgrade every process sharing a queue at once, and do not roll back below this version while delivery tasks are still queued.

## delivery is the third outbound path, and it never adds a recipient

[delivery](delivery/) is the pipeline that Emissary's outbox is moving to. Its fan-out is a stack of `iter.Seq` stages, and it must stay that way: no channels, no goroutines, no recursion, and no stage that feeds an earlier one.

- **Addressing belongs to the application.** The package delivers to whoever the activity names. It does not add a followers collection to a `Delete`, or a parent's author to a reply.
- **Every delivery goes to the actor's own `inbox`.** `PreferredInbox` and `sharedInbox` are never read here. A shared inbox cannot carry a followers-only or circle-only post, so the application chose one request per recipient.
- **An inbox on a different origin than its actor is refused, and reported.** Without that check, a crowd of fake followers could each name a victim's server as their inbox. The check is `uri.NotSameOrigin`, which compares scheme, host and port. `HostOf` is a different thing: it is the key a host's health is kept under, it drops the port, and it must never be used for this check.
- **The package imports no circuit breaker, and the send task reports nothing.** The application passes `WithBlockedHosts` for the fan-out and `WithRemoteOptions` for each request. An option such as floodgate's records every outcome itself, so `Pipeline.result` only reads the error. Adding a report there would count each outcome twice, and re-recording a breaker's own 429 would move its pause later on every waiting delivery.
- **`result` reads status codes, whoever sent them.** 423 means "this host gets nothing" and ends the task quietly; 429 reschedules for the wait the error names, capped at one hour; a 5xx or a timeout is retried. A breaker's refusal and a server's answer take the same path.
- **A rate-limited delivery ends by age, not by count.** turbine sets `RetryCount` to zero on every requeue, so a host that always answers 429 would hold its deliveries forever. Each send task carries `queuedAt`, written once at fan-out, and a 429 on a task older than `maximumDeliveryAge` (seven days) is a `Failure`. `sender` has the same rule and the same one-hour cap. A task with no `queuedAt`, queued by an older version, never expires.
- **A remote collection is never expanded.** It stands for its owner, who is looked for among the authors of the objects the activity replies to and the people it names, in the same send task. The search loads at most `maxReplyParents` posts and `maxOwnerCandidates` actors, because those documents come from other servers. A candidate owns the collection only when the two share an origin; otherwise anyone the activity names could claim another actor's followers and take the copy meant for that actor.
- **`StripBlindRecipients` is `deep.Copy` followed by `deep.Remove`, from [rosetta/deep](https://github.com/benpate/rosetta/tree/main/deep).** The copy shares no map or slice with the original and keeps the type of every value, so it can be stored as well as sent. A value the walk cannot search is an error and nothing is returned to send: a struct, a pointer, a function, a channel, a `json.RawMessage`, or a document that contains itself. Any of those could carry a blind recipient out, so do not catch the error and send the original. The fan-out still reads `bto` and `bcc` from the original to address the delivery, and the proof is attached after the strip, because a proof made over the blind recipients fails once they are gone.- **`delivery` reads an activity through a `streams.Document` made without a client.** `FanOut` wraps the task's activity once, and the stages read each property with `Document.IDs` and `RangeIDs`, which handle a URL, an object with an `id` or `@id`, a list, and the named map and slice types a database driver returns. Nothing in the fan-out can load, because the Document has `NilClient`. `ownerCandidates` reads a loaded parent, which does have a client, so it reads `attributedTo` and `actor` with `Get` and never with `Actor()`, which would fetch each author. A reference with no id is skipped. `Document.ID` removes HTML from an id, so a recipient URL is sent as sanitized.
- **A collection that cannot be read fails the whole fan-out before anything is queued.** A partial list followed by a retry would deliver to some people twice and never tell which.
- **Send tasks are signed by `activity id + recipient`,** so a retried fan-out queues no second task for a recipient whose first is still waiting. The queue frees a signature when its task finishes, so a recipient already delivered to is sent a second copy; receiving servers drop a repeated activity id.
- **The digests in a send task are not read yet.** `sigs.Sign` computes `Digest` from the body on every attempt. The carried `digest` and `contentDigest` are there for when `sigs` can sign with RFC 9421, which it cannot today.
- **A 429 with no `Retry-After` reschedules the delivery for a minute,** because `derp.IsTooManyRequests` answers one minute when the response names no wait. It was an hour before derp changed its default on 2026-11-12.

## Timestamp formats: AS2 in datetime, HTTP Date in sigs

AS2 properties use RFC3339 via the [datetime](datetime/) package; the IMF-fixdate formatter for the HTTP `Date` header is unexported inside [sigs](sigs/) ON PURPOSE, so document-building code can never grab the wrong one. Don't export it or add an HTTP date helper to datetime. Details in [datetime/AGENTS.md](datetime/AGENTS.md).

## property package coercion rules worth knowing

- **Every scalar behaves as a one-element list:** `Head()` returns itself, `Tail()` returns `Nil`, `Len()` is 1. `Map.Len()` is also 1 — a map is one entity, not a count of its keys.
- **A bare string is treated as an object reference:** `String.Get` answers only `id` (returning itself); setting any property on a String promotes it to `Map{"id": old}`.
- **Empty means nil:** `String("")` and a zero-length `Map` report `IsNil()`, and `Document.Get` returns `NilDocument` for nil values — so a property stored as `""` reads back as missing.
- **`Nil.Set` returns a `Map` that holds the property,** the same promotion a scalar makes. It used to return `Nil`, which discarded the value and reported nothing.
- **`NewValue` reads every integer kind.** A document read back from MongoDB can hold an `int32`, which fell through to `Nil` and read as zero. A `uint64` above `MaxInt64` becomes a `Float`, and a `json.Number` becomes an `Int64` or a `Float`.

## `ContextTypeToot` must use Mastodon's own prefixes: `http://schema.org#` and `http://joinmastodon.org/ns#`

The canonical schema.org IRI (`https://schema.org/`) looks like the correct value and is not. Software that expands JSON-LD looks profile-field values up by their expanded IRI, so `value` must expand to exactly what Mastodon publishes. Funkwhale accepts only `http://schema.org#value` and the Litepub equivalent, so any actor with a `PropertyValue` attachment failed its validation (`attachment.value: This field is required`) and could not be followed. `a5c1589` made that change in May 2026, and it shipped from v0.17.7 until it was reverted.

Plain-JSON readers (Mastodon, Misskey, Pleroma, Pixelfed, Lemmy, PeerTube, WriteFreely, WordPress, NodeBB) match the literal `PropertyValue` and `value` keys and never read the prefix, so the change is invisible to them. Funkwhale is the case that catches a regression: expand an actor with pyld 2.0.4 and check that its attachment carries `http://schema.org#value`.

The `toot` prefix has the same trap. Mastodon's `app/helpers/context_helper.rb` writes `'toot' => 'http://joinmastodon.org/ns#'`, with `http`, in every one of its context entries (read 2026-11-12). `https://joinmastodon.org/ns#` expands `discoverable` and `indexable` to IRIs that no JSON-LD processor matches.

## Collections are read and served by different packages, and both have rules

- **`streams.OrderedCollection` and `OrderedCollectionPage` read `orderedItems`,** and fall back to `items` only when it is missing, for servers that write the wrong name. Both collection types set their `ID`.
- **`streams.ContextEntry` has an `UnmarshalJSON`,** so a `Context` reads the plain string form, the object form, and a list of either. A term whose definition is an object, such as `{"@id": …}`, does not fit in `Extensions` and is skipped, so a context does not survive a round trip unchanged when it holds one.
- **`streams.Image` returns no raw remote strings.** `Href` and `URL` return an absolute `http` or `https` address or nothing, and `Summary` and `MediaType` go through the same sanitizer as `Document.String`. An image whose address is refused reports `IsNil`.
- **[collection](collection/) takes an `http.ResponseWriter` and an `*http.Request`,** and the module no longer depends on echo. Every response, the collection and its pages alike, is `application/activity+json`. The `after` cursor is an item's id, so it is query-escaped in `first`, `next` and the page's `id`. A full page whose last item has no id names no `next`, because an empty cursor reads as a request for the collection itself.
- **`collections.CountItems` takes the same options as `RangePages`,** and returns an error when it recovers from a panic. It used to return `0, nil`, which reported a failure as an empty collection.
