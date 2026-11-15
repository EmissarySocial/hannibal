# Hannibal / validator

Validator provides pluggable checks that confirm an inbound ActivityPub activity is authentic before the [router](../router) hands it to your application. The router runs a chain of validators; each one returns a `Result` of `ResultValid`, `ResultInvalid`, or `ResultUnknown`.

```go
// Build a chain of validators (this is what router uses by default)
validators := []router.Validator{
	validator.NewHTTPSig(nil),       // verify the HTTP Signature
	validator.NewDeletedObject(),    // confirm Delete activities really are deleted
}
```

## Results

A validator returns one of three results, and the router walks the chain accordingly:

- **`ResultValid`** — the activity is authentic; stop and accept it.
- **`ResultInvalid`** — the activity is forged or malformed; stop and reject it.
- **`ResultUnknown`** — this validator can't decide; continue to the next validator. If every validator returns `ResultUnknown`, validation fails closed.

## Included Validators

- **`HTTPSig`** verifies the request's HTTP Signature against the actor's public key. It returns `ResultUnknown` when the key cannot be found, so that the next validator decides, and `ResultInvalid` when the signature does not match.
- **`MatchActor`** confirms the activity's actor matches an expected actor ID.
- **`DeletedObject`** confirms a `Delete` activity refers to an object that is actually gone. It fetches the object's URL, and the `Delete` is valid when the answer is 404, 410, or a `Tombstone`. Any other document means the object still exists, and the `Delete` is invalid.
- **`None`** accepts every activity without checking it (always `ResultValid`). Use it only on routes that authenticate requests some other way, such as with a cookie or an API key.
