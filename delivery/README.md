# delivery

Turns one outbound ActivityPub activity into deliveries to the people it names. It replaces the fan-out in [sender](../sender/) for applications that want a circuit breaker, retries that last a week, and their own rules for who receives what.

## How it works

Two queue tasks do the work, and `Consumer` runs both on a [turbine](https://github.com/benpate/turbine) queue.

**`Delivery:FanOut`** takes an activity under the argument `activity`, with its `bto` and `bcc` still in place, and runs it through a stack of iterators:

1. **Extract** every recipient in `to`, `cc`, `bto` and `bcc`, with the property it came from.
2. **Expand relays:** a Public marker in `to` becomes the application's relays. A marker anywhere else is dropped.
3. **Expand collections:** each of the application's own collections becomes its members, one level deep.
4. **Deduplicate** by URL.
5. **Skip blocked hosts,** from a list the application returns once per fan-out.
6. **Filter** with the application's own test, built once per fan-out.

It then removes `bto` and `bcc` at every level, attaches an integrity proof if the application supplies one, serializes the activity once, and queues one `Delivery:Send` task per recipient. A recipient whose URL begins with `mailto:` goes to the application's email handler.

**`Delivery:Send`** delivers to one recipient:

1. **Resolve** the recipient through the application's client. A remote collection is never expanded; it stands for its owner, who is found among the actors that the activity points to.
2. **Check the inbox:** an inbox on a different origin than its actor is refused.
3. **Sign and deliver** to the recipient's own inbox, with whatever `remote` options the application adds.
4. **Choose the task's result** from the error that comes back.

## What the application supplies

```go
pipeline := delivery.New(taskQueue, client, signers,
    delivery.WithBlockedHosts(blockedHosts),   // hosts that nothing is queued for
    delivery.WithRemoteOptions(remoteOptions), // options for each request, such as a circuit breaker's
    delivery.WithCollections(collections), // the application's followers and other collections
    delivery.WithRelays(relays),
    delivery.WithFilter(filter),           // block lists, loaded once per fan-out
    delivery.WithFilterOne(filterOne),     // the same rules, for one recipient found later
    delivery.WithEmail(email),
    delivery.WithProof(proof),
)

queue.New(queue.WithConsumers(delivery.Consumer(pipeline)))
```

Only the queue, the client and the signers are required. The package never adds a recipient to an activity: addressing belongs to the application.

## A circuit breaker is the application's to add

This package does not import one. `WithBlockedHosts` returns the hosts that the fan-out skips, and `WithRemoteOptions` returns the `remote` options added to each delivery's request. A breaker such as [floodgate](https://github.com/benpate/floodgate) plugs into both: its `remotegate.Option` asks before each request, refuses the ones it must hold back, and records every outcome for itself. The send task reports nothing; it reads the error and chooses a result.

`WithRemoteOptions` is called once per delivery, so an option may keep state between its hooks.

## Results

The send task reads only the error's status code and wait, so an option that refuses a request gets the same treatment as a server that answers with that code.

| The error says | The task |
| --- | --- |
| nothing (2xx) | succeeds |
| 423 | succeeds without delivering: the host gets nothing, and nothing is held for it |
| 429 | is rescheduled for the wait it names, between one second and one hour, without spending a retry. Once the delivery was first queued more than seven days ago, it fails for good |
| 408, 5xx, a timeout, or no answer | is retried |
| 404 or 410 | fails for good |
| any other 4xx | fails for good |

A send task is retried 17 times, doubling its wait up to one day, which covers just over seven days.

A task whose signing actor cannot be loaded fails for good when the application's `signers` function answers 404 or 410, and is retried on any other error. A key that cannot sign fails for good, and nothing is sent.
