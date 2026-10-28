package sender

import "crypto"

// testActor is a stub Actor with a fixed ID and no private key.
type testActor struct{}

// ActorID returns the stub actor's fixed URL.
func (t testActor) ActorID() string {
	return "https://test.actor.social"
}

// PrivateKey returns a fixed key ID and a nil private key.
func (t testActor) PrivateKey() (string, crypto.PrivateKey) {
	return "https://test.actor.social/#main-key", nil
}
