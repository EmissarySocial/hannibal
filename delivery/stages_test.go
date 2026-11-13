package delivery

import (
	"errors"
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/benpate/hannibal/vocab"
	"github.com/stretchr/testify/require"
)

const (
	testActor     = "https://example.com/@author"
	testFollowers = "https://example.com/@author/pub/followers"
	testCircle    = "https://example.com/@author/pub/circles/1"
	testAlice     = "https://remote.example/@alice"
	testBob       = "https://remote.example/@bob"
	testCarol     = "https://other.example/@carol"
	testRelay     = "https://relay.example/actor"
)

// testCollections is a Collections that knows a fixed set of local collections
type testCollections struct {
	members map[string][]string
	errors  map[string]error
	loaded  []string
}

// IsCollection returns TRUE for any URL beneath the local actor's collections
func (collections *testCollections) IsCollection(collectionURL string) bool {
	return strings.HasPrefix(collectionURL, testActor+"/pub/")
}

// Members returns the members of a known collection, and records that it was asked
func (collections *testCollections) Members(collectionURL string) (iter.Seq[string], error) {

	collections.loaded = append(collections.loaded, collectionURL)

	if err := collections.errors[collectionURL]; err != nil {
		return nil, err
	}

	return slices.Values(collections.members[collectionURL]), nil
}

// hostSet is a HostList backed by a map
type hostSet map[string]bool

// Contains returns TRUE if the host is in the set
func (hosts hostSet) Contains(host string) bool {
	return hosts[host]
}

// TestExtract verifies that every recipient is yielded with its property, in every shape that
// a property value can take, and that nothing is dropped
func TestExtract(t *testing.T) {

	activity := map[string]any{
		"to":  []any{vocab.NamespacePublic, map[string]any{"id": testAlice, "type": "Person"}},
		"cc":  testFollowers,
		"bto": []string{testBob},
		"bcc": []any{testCarol, 42, map[string]any{"type": "Person"}},
		"object": map[string]any{
			"to": []any{"https://example.com/ignored"},
		},
	}

	require.Equal(t, []Recipient{
		{URL: vocab.NamespacePublic, Field: FieldTo},
		{URL: testAlice, Field: FieldTo},
		{URL: testFollowers, Field: FieldCC},
		{URL: testBob, Field: FieldBTo},
		{URL: testCarol, Field: FieldBCC},
	}, slices.Collect(Extract(activity)))

	require.Empty(t, slices.Collect(Extract(map[string]any{})))
	require.Empty(t, slices.Collect(Extract(nil)))
}

// TestExtract_StopsEarly verifies that the iterator stops when its consumer does
func TestExtract_StopsEarly(t *testing.T) {

	activity := map[string]any{"to": []any{testAlice, testBob}, "cc": []any{testCarol}}

	for recipient := range Extract(activity) {
		require.Equal(t, testAlice, recipient.URL)
		break
	}
}

// TestExpandRelays verifies that a Public marker in `to` becomes the relays, once, and that a
// marker anywhere else adds none
func TestExpandRelays(t *testing.T) {

	relays := slices.Values([]string{testRelay, "https://relay2.example/actor"})

	collect := func(recipients ...Recipient) []Recipient {
		return slices.Collect(ExpandRelays(slices.Values(recipients), relays))
	}

	// Listed: Public in `to`, in any spelling, and only once however often it appears
	for _, public := range []string{vocab.NamespaceActivityStreamsPublic, vocab.NamespaceASPublic, vocab.NamespacePublic} {
		require.Equal(t, []Recipient{
			{URL: testRelay, Field: FieldRelay},
			{URL: "https://relay2.example/actor", Field: FieldRelay},
			{URL: testFollowers, Field: FieldCC},
		}, collect(
			Recipient{URL: public, Field: FieldTo},
			Recipient{URL: public, Field: FieldTo},
			Recipient{URL: testFollowers, Field: FieldCC},
		), public)
	}

	// Unlisted, or blind: the marker is dropped and no relay is added
	for _, field := range []string{FieldCC, FieldBTo, FieldBCC} {
		require.Equal(t, []Recipient{{URL: testAlice, Field: FieldTo}}, collect(
			Recipient{URL: testAlice, Field: FieldTo},
			Recipient{URL: vocab.NamespacePublic, Field: field},
		), field)
	}

	// Private: nothing changes
	require.Equal(t, []Recipient{{URL: testAlice, Field: FieldTo}}, collect(Recipient{URL: testAlice, Field: FieldTo}))
}

// TestExpandRelays_NotReadUnlessPublic verifies that the relay list is not touched for an
// activity that is not public
func TestExpandRelays_NotReadUnlessPublic(t *testing.T) {

	relays := func(func(string) bool) {
		t.Fatal("the relay list must not be read for a private activity")
	}

	recipients := slices.Values([]Recipient{{URL: testAlice, Field: FieldTo}, {URL: vocab.NamespacePublic, Field: FieldCC}})
	require.Len(t, slices.Collect(ExpandRelays(recipients, relays)), 1)
}

// TestExpandCollections verifies that local collections become their members, one level deep,
// and that everything else passes through
func TestExpandCollections(t *testing.T) {

	collections := &testCollections{members: map[string][]string{
		testFollowers: {testAlice, "mailto:dana@example.org", testCircle},
		testCircle:    {testBob},
	}}

	recipients := slices.Values([]Recipient{
		{URL: testCarol, Field: FieldTo},
		{URL: testFollowers, Field: FieldCC},
		{URL: "https://remote.example/@alice/followers", Field: FieldCC},
	})

	result := slices.Collect(ExpandCollections(recipients, collections, func(err error) { t.Fatal(err) }))

	require.Equal(t, []string{
		testCarol,
		testAlice,
		"mailto:dana@example.org",
		"https://remote.example/@alice/followers",
	}, result, "the nested circle is dropped, and the remote collection is not expanded")

	require.Equal(t, []string{testFollowers}, collections.loaded, "a nested collection is never loaded")
}

// TestExpandCollections_Error verifies that a failed collection is reported and yields nothing,
// while the other recipients are still read
func TestExpandCollections_Error(t *testing.T) {

	failure := errors.New("database is down")
	collections := &testCollections{
		members: map[string][]string{testCircle: {testBob}},
		errors:  map[string]error{testFollowers: failure},
	}

	recipients := slices.Values([]Recipient{
		{URL: testFollowers, Field: FieldTo},
		{URL: testCircle, Field: FieldTo},
	})

	var reported []error
	result := slices.Collect(ExpandCollections(recipients, collections, func(err error) { reported = append(reported, err) }))

	require.Equal(t, []string{testBob}, result)
	require.Equal(t, []error{failure}, reported)
}

// TestDeduplicate verifies that each URL is yielded once, in first-seen order, and that an
// email address and an actor are separate recipients
func TestDeduplicate(t *testing.T) {

	input := slices.Values([]string{testAlice, "", testBob, testAlice, "mailto:alice@remote.example", testBob, "mailto:alice@remote.example"})

	require.Equal(t, []string{testAlice, testBob, "mailto:alice@remote.example"}, slices.Collect(Deduplicate(input)))
}

// TestSkipBlockedHosts verifies that recipients on a blocked host are dropped, and that email passes
func TestSkipBlockedHosts(t *testing.T) {

	blocked := hostSet{"remote.example": true, "example.org": true}
	input := slices.Values([]string{testAlice, testCarol, "mailto:dana@example.org", "https://REMOTE.example/@upper"})

	require.Equal(t, []string{testCarol, "mailto:dana@example.org"}, slices.Collect(SkipBlockedHosts(input, blocked)))
}

// TestFilter verifies that only allowed recipients are yielded, and that each is checked once
func TestFilter(t *testing.T) {

	checked := make([]string, 0)

	allowed := func(recipientURL string) bool {
		checked = append(checked, recipientURL)
		return recipientURL != testBob
	}

	input := slices.Values([]string{testAlice, testBob, testCarol})

	require.Equal(t, []string{testAlice, testCarol}, slices.Collect(Filter(input, allowed)))
	require.Equal(t, []string{testAlice, testBob, testCarol}, checked)
}

// TestStages_FollowerInTwoCircles verifies the pipeline's central promise: someone who follows
// and is in two circles is sent one copy
func TestStages_FollowerInTwoCircles(t *testing.T) {

	const secondCircle = "https://example.com/@author/pub/circles/2"

	collections := &testCollections{members: map[string][]string{
		testFollowers: {testAlice, testBob},
		testCircle:    {testAlice},
		secondCircle:  {testAlice, testCarol},
	}}

	activity := map[string]any{
		"to":  []any{testFollowers, testCircle},
		"bcc": []any{secondCircle, testAlice},
	}

	recipients := ExpandRelays(Extract(activity), slices.Values([]string{testRelay}))
	expanded := ExpandCollections(recipients, collections, func(err error) { t.Fatal(err) })

	require.Equal(t, []string{testAlice, testBob, testCarol}, slices.Collect(Deduplicate(expanded)))
}

// TestHostOf verifies that a host is lower case, has no port, and is empty for a value without one
func TestHostOf(t *testing.T) {
	require.Equal(t, "remote.example", HostOf("https://Remote.Example/@alice"))
	require.Equal(t, "remote.example", HostOf("HTTPS://REMOTE.EXAMPLE/@alice"))
	require.Equal(t, "remote.example", HostOf("https://remote.example:8443/inbox"))
	require.Equal(t, "remote.example", HostOf("https://user:secret@remote.example/inbox"))
	require.Empty(t, HostOf("mailto:alice@remote.example"))
	require.Empty(t, HostOf("not a url"))
	require.Empty(t, HostOf("://bad"))
}

// TestIsEmail verifies that only a mailto: URL counts as email
func TestIsEmail(t *testing.T) {
	require.True(t, IsEmail("mailto:alice@remote.example"))
	require.False(t, IsEmail(testAlice))
	require.False(t, IsEmail(""))
}
