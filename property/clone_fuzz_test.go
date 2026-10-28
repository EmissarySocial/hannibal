package property

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/require"
)

// The two invariants every clone must satisfy, whatever the input shape:
//
//  1. EQUALITY  -- the clone is deeply equal to the original, with concrete types preserved.
//  2. INDEPENDENCE -- the clone shares no mutable container with the original, at any depth.
//
// Independence is checked structurally, by walking both trees in parallel and asserting that no
// container in one is the same underlying object as its counterpart in the other. That is
// stronger than mutate-and-compare: it catches sharing even in branches a mutation never reaches.

// FuzzClone_JSON drives the clone with arbitrary JSON documents -- the shape this package actually
// sees on the wire. JSON yields map[string]any, []any, string, float64, bool, and nil.
func FuzzClone_JSON(f *testing.F) {

	f.Add([]byte(`{}`))
	f.Add([]byte(`{"id":"https://example.com/1","type":"Note"}`))
	f.Add([]byte(`{"object":{"tag":[{"type":"Mention","href":"x"}]}}`))
	f.Add([]byte(`{"a":[[[{"b":[1,2,{"c":null}]}]]]}`))
	f.Add([]byte(`{"to":["a","b"],"cc":[],"bcc":{}}`))
	f.Add([]byte(`{"n":1e308,"neg":-0.0,"big":9007199254740993}`))
	f.Add([]byte("{\"unicode\":\"\\u0000\\ud83d\\ude00\",\"empty\":\"\"}"))
	f.Add([]byte(`[1,2,3]`))

	f.Fuzz(func(t *testing.T, data []byte) {

		var parsed any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Skip() // not JSON; nothing to say about it
		}

		switch typed := parsed.(type) {

		case map[string]any:
			assertCloneInvariants(t, Map(typed), Map(typed).Clone())

		case []any:
			assertCloneInvariants(t, Slice(typed), Slice(typed).Clone())

		default:
			t.Skip() // scalar top level; Map/Slice clone is not involved
		}
	})
}

// FuzzClone_Structure builds documents from the fuzz input using every container and scalar type
// cloneChild knows about -- including the ones JSON cannot express (int64, mapof.Any, sliceof.Any,
// property.Value children, byte slices, and named container types that hit the reflection
// fallback).
func FuzzClone_Structure(f *testing.F) {

	f.Add([]byte{0}, uint8(1))
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8}, uint8(3))
	f.Add([]byte{9, 4, 2, 7, 15, 1, 6, 3, 11, 8, 13, 0}, uint8(4))
	f.Add([]byte{15, 15, 15, 15, 15, 15}, uint8(5))
	f.Add([]byte("arbitrary bytes drive the generator"), uint8(4))

	f.Fuzz(func(t *testing.T, seed []byte, depth uint8) {

		if len(seed) == 0 {
			t.Skip()
		}

		// Cap the depth so the generator cannot build something pathological enough to make the
		// test itself the bottleneck. TestClone_DeeplyNested covers real depth separately.
		generator := &structureGenerator{seed: seed, maxDepth: int(depth % 6)}
		original := generator.buildMap(0)

		assertCloneInvariants(t, Map(original), Map(original).Clone())
	})
}

/******************************************
 * Invariant checking
 ******************************************/

// assertCloneInvariants asserts both clone invariants: deep equality and total container
// independence.
func assertCloneInvariants(t *testing.T, original Value, clone Value) {
	t.Helper()

	originalRaw := original.Raw()
	cloneRaw := clone.Raw()

	require.IsType(t, original, clone, "Clone must preserve the container's own type")
	require.True(t, equivalent(originalRaw, cloneRaw),
		"clone must carry the same content as the original\noriginal: %#v\nclone:    %#v", originalRaw, cloneRaw)

	assertNoSharedContainers(t, originalRaw, cloneRaw, "$")
}

// equivalent reports whether two values carry the same content, with concrete types preserved.
//
// There is exactly one permitted type difference, and it is the documented one: cloneChild's
// reflection fallback cannot reconstruct an arbitrary named container type, so a named map or
// slice (primitive.A, primitive.M, map[string]string) legitimately clones to the plain
// map[string]any / []any. Every other type change is a defect.
func equivalent(original any, clone any) bool {

	originalValue := reflect.ValueOf(original)
	cloneValue := reflect.ValueOf(clone)

	if !originalValue.IsValid() || !cloneValue.IsValid() {
		return originalValue.IsValid() == cloneValue.IsValid()
	}

	switch originalValue.Kind() {

	case reflect.Map:
		if !typeIsPreservedOrNormalized(originalValue.Type(), cloneValue.Type(), reflect.TypeOf(map[string]any(nil))) {
			return false
		}

		if originalValue.Len() != cloneValue.Len() {
			return false
		}

		for iterator := originalValue.MapRange(); iterator.Next(); {
			if cloneChild := cloneValue.MapIndex(iterator.Key()); !cloneChild.IsValid() || !equivalent(iterator.Value().Interface(), cloneChild.Interface()) {
				return false
			}
		}

		return true

	case reflect.Slice:
		// Byte slices are data: they must come through byte-for-byte, and with their own type.
		if originalValue.Type().Elem().Kind() == reflect.Uint8 {
			return reflect.DeepEqual(original, clone)
		}

		if !typeIsPreservedOrNormalized(originalValue.Type(), cloneValue.Type(), reflect.TypeOf([]any(nil))) {
			return false
		}

		if originalValue.Len() != cloneValue.Len() {
			return false
		}

		for index := range originalValue.Len() {
			if !equivalent(originalValue.Index(index).Interface(), cloneValue.Index(index).Interface()) {
				return false
			}
		}

		return true
	}

	return reflect.DeepEqual(original, clone)
}

// assertNoSharedContainers walks two structurally identical trees in parallel, failing if any
// container in one is the same underlying object as its counterpart in the other.
func assertNoSharedContainers(t *testing.T, original any, clone any, path string) {
	t.Helper()

	originalValue := reflect.ValueOf(original)
	cloneValue := reflect.ValueOf(clone)

	if !originalValue.IsValid() || !cloneValue.IsValid() {
		return
	}

	switch originalValue.Kind() {

	case reflect.Map:
		// A byte slice is data, and an empty map may legitimately share the zero-size allocation,
		// so only non-empty maps are required to be distinct objects.
		if originalValue.Len() > 0 {
			require.NotEqual(t, originalValue.Pointer(), cloneValue.Pointer(),
				"map at %s is shared between original and clone", path)
		}

		for iterator := originalValue.MapRange(); iterator.Next(); {
			key := iterator.Key()
			cloneChild := cloneValue.MapIndex(key)
			require.True(t, cloneChild.IsValid(), "clone is missing key %v at %s", key, path)
			assertNoSharedContainers(t, iterator.Value().Interface(), cloneChild.Interface(),
				fmt.Sprintf("%s.%v", path, key))
		}

	case reflect.Slice:
		// Byte slices are passed through by design -- they are data, not containers.
		if originalValue.Type().Elem().Kind() == reflect.Uint8 {
			return
		}

		if originalValue.Len() > 0 {
			require.NotEqual(t, originalValue.Pointer(), cloneValue.Pointer(),
				"slice at %s is shared between original and clone", path)
		}

		for index := range originalValue.Len() {
			assertNoSharedContainers(t, originalValue.Index(index).Interface(),
				cloneValue.Index(index).Interface(), fmt.Sprintf("%s[%d]", path, index))
		}
	}
}

// typeIsPreservedOrNormalized allows a cloned container to keep its original type, or to have
// been normalized to the plain container type by the reflection fallback.
func typeIsPreservedOrNormalized(originalType reflect.Type, cloneType reflect.Type, plainType reflect.Type) bool {
	return (cloneType == originalType) || (cloneType == plainType)
}

/******************************************
 * Structure generator
 ******************************************/

// structureGenerator builds a deterministic, type-diverse document from a byte seed. Each byte
// consumed selects the next value's type, so a fuzzer mutating the seed explores different
// container/scalar combinations rather than different JSON text.
type structureGenerator struct {
	seed     []byte
	cursor   int
	maxDepth int
}

// buildMap creates a map with a seed-determined number of entries.
func (generator *structureGenerator) buildMap(depth int) map[string]any {

	result := make(map[string]any)
	count := int(generator.next() % 5)

	for index := range count {
		result[fmt.Sprintf("key%d", index)] = generator.buildValue(depth + 1)
	}

	return result
}

// next returns the next seed byte, cycling when the seed runs out so generation always terminates.
func (generator *structureGenerator) next() byte {
	value := generator.seed[generator.cursor%len(generator.seed)]
	generator.cursor++
	return value
}

// buildValue creates one value, choosing its type from the next seed byte. Past maxDepth it emits
// scalars only, so recursion always bottoms out.
func (generator *structureGenerator) buildValue(depth int) any {

	choice := generator.next()

	if depth >= generator.maxDepth {
		return generator.buildScalar(choice)
	}

	switch choice % 12 {

	case 0:
		return generator.buildMap(depth)

	case 1:
		return generator.buildSlice(depth)

	case 2:
		return mapof.Any(generator.buildMap(depth))

	case 3:
		return sliceof.Any(generator.buildSlice(depth))

	case 4:
		return Map(generator.buildMap(depth))

	case 5:
		return Slice(generator.buildSlice(depth))

	case 6:
		return namedMap(generator.buildMap(depth))

	case 7:
		return namedSlice(generator.buildSlice(depth))

	default:
		return generator.buildScalar(choice)
	}
}

// buildScalar creates one leaf value, covering the scalar types a document can carry.
func (generator *structureGenerator) buildScalar(choice byte) any {

	switch choice % 9 {

	case 0:
		return nil

	case 1:
		return fmt.Sprintf("string-%d", choice)

	case 2:
		return int64(choice) * 1_000_000_000

	case 3:
		return int(choice)

	case 4:
		return float64(choice) / 3

	case 5:
		return choice%2 == 0

	case 6:
		return []byte{choice, choice + 1}

	case 7:
		return String(fmt.Sprintf("propertyString-%d", choice))

	default:
		return Int64(int64(choice))
	}
}

// buildSlice creates a slice with a seed-determined number of elements.
func (generator *structureGenerator) buildSlice(depth int) []any {

	count := int(generator.next() % 4)
	result := make([]any, 0, count)

	for range count {
		result = append(result, generator.buildValue(depth+1))
	}

	return result
}
