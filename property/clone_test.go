package property

import (
	"testing"
	"time"

	"github.com/benpate/rosetta/mapof"
	"github.com/benpate/rosetta/sliceof"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// namedSlice and namedMap stand in for containers like the Mongo driver's primitive.A and
// primitive.M: named types whose underlying type is a container, which a type switch cannot match
// and which therefore exercise cloneChild's reflection fallback. They are declared locally so the
// property package does not take a dependency on the Mongo driver just to test this.
type namedSlice []any

// namedMap is a named map type, standing in for the Mongo driver's primitive.M.
type namedMap map[string]any

/******************************************
 * The deep-copy contract
 ******************************************/

// TestMap_CloneIsDeep is the contract test for Value.Clone: "returns a deep copy". The original
// TestMap_Clone used a flat fixture, so a shallow clone satisfied it while nested containers
// stayed shared.
func TestMap_CloneIsDeep(t *testing.T) {

	original := Map{
		"scalar": "unchanged",
		"object": map[string]any{
			"nested": "original",
			"deeper": map[string]any{"nested": "original"},
		},
		"array": []any{
			map[string]any{"nested": "original"},
		},
	}

	clone := original.Clone().(Map)

	// Mutate every nested container in the clone.
	clone["object"].(map[string]any)["nested"] = "MUTATED"
	clone["object"].(map[string]any)["deeper"].(map[string]any)["nested"] = "MUTATED"
	clone["array"].([]any)[0].(map[string]any)["nested"] = "MUTATED"

	// The original must be untouched at every level.
	object := original["object"].(map[string]any)
	assert.Equal(t, "original", object["nested"])
	assert.Equal(t, "original", object["deeper"].(map[string]any)["nested"])
	assert.Equal(t, "original", original["array"].([]any)[0].(map[string]any)["nested"])
	assert.Equal(t, "unchanged", original["scalar"])
}

// TestSlice_CloneIsDeep is the Slice half of the same contract.
func TestSlice_CloneIsDeep(t *testing.T) {

	original := Slice{
		"scalar",
		map[string]any{"nested": "original"},
		[]any{map[string]any{"nested": "original"}},
	}

	clone := original.Clone().(Slice)

	clone[1].(map[string]any)["nested"] = "MUTATED"
	clone[2].([]any)[0].(map[string]any)["nested"] = "MUTATED"

	assert.Equal(t, "original", original[1].(map[string]any)["nested"])
	assert.Equal(t, "original", original[2].([]any)[0].(map[string]any)["nested"])
	assert.Equal(t, "scalar", original[0])
}

/******************************************
 * cloneChild: one test per branch
 ******************************************/

// TestCloneChild_ByteSlice confirms byte slices survive as byte slices. They are checked ahead of
// the reflection fallback, which sees a slice Kind and would shred them into []any.
func TestCloneChild_ByteSlice(t *testing.T) {

	original := Map{"bytes": []byte("kept")}
	clone := original.Clone().(Map)

	require.IsType(t, []byte(nil), clone["bytes"])
	assert.Equal(t, []byte("kept"), clone["bytes"])
}

// TestCloneChild_Value confirms a child that is itself a property.Value clones through its own
// Clone method and keeps its concrete type.
func TestCloneChild_Value(t *testing.T) {

	original := Map{
		"propertyMap":   Map{"nested": "original"},
		"propertySlice": Slice{map[string]any{"nested": "original"}},
		"propertyNil":   Nil{},
		"propertyInt64": Int64(7),
	}

	clone := original.Clone().(Map)

	clonedMap, ok := clone["propertyMap"].(Map)
	require.True(t, ok, "a Value child must keep its concrete type")
	clonedMap["nested"] = "MUTATED"

	clonedSlice, ok := clone["propertySlice"].(Slice)
	require.True(t, ok, "a Value child must keep its concrete type")
	clonedSlice[0].(map[string]any)["nested"] = "MUTATED"

	assert.Equal(t, "original", original["propertyMap"].(Map)["nested"])
	assert.Equal(t, "original", original["propertySlice"].(Slice)[0].(map[string]any)["nested"])
	assert.Equal(t, Nil{}, clone["propertyNil"])
	assert.Equal(t, Int64(7), clone["propertyInt64"])
}

// TestCloneChild_PlainContainers covers the map[string]any and []any branches.
func TestCloneChild_PlainContainers(t *testing.T) {

	original := Map{
		"map":   map[string]any{"nested": "original"},
		"slice": []any{"original"},
	}

	clone := original.Clone().(Map)

	require.IsType(t, map[string]any{}, clone["map"])
	require.IsType(t, []any{}, clone["slice"])

	clone["map"].(map[string]any)["nested"] = "MUTATED"
	clone["slice"].([]any)[0] = "MUTATED"

	assert.Equal(t, "original", original["map"].(map[string]any)["nested"])
	assert.Equal(t, "original", original["slice"].([]any)[0])
}

// TestCloneChild_RosettaContainers covers the mapof.Any and sliceof.Any branches, and asserts the
// concrete type is preserved rather than normalized to map[string]any / []any.
func TestCloneChild_RosettaContainers(t *testing.T) {

	original := Map{
		"mapofAny":   mapof.Any{"nested": "original"},
		"sliceofAny": sliceof.Any{map[string]any{"nested": "original"}},
	}

	clone := original.Clone().(Map)

	clonedMap, ok := clone["mapofAny"].(mapof.Any)
	require.True(t, ok, "mapof.Any must clone to mapof.Any")

	clonedSlice, ok := clone["sliceofAny"].(sliceof.Any)
	require.True(t, ok, "sliceof.Any must clone to sliceof.Any")

	clonedMap["nested"] = "MUTATED"
	clonedSlice[0].(map[string]any)["nested"] = "MUTATED"

	assert.Equal(t, "original", original["mapofAny"].(mapof.Any)["nested"])
	assert.Equal(t, "original", original["sliceofAny"].(sliceof.Any)[0].(map[string]any)["nested"])
}

// TestCloneChild_WaywardContainers covers the two reflection fallbacks: named and
// differently-typed containers that no type-switch case can match. primitive.A is the one that
// matters in practice -- it is what the Mongo driver hands back for a BSON array.
func TestCloneChild_WaywardContainers(t *testing.T) {

	original := Map{
		"namedSlice":  namedSlice{map[string]any{"nested": "original"}},
		"namedMap":    namedMap{"nested": "original"},
		"mapString":   map[string]string{"nested": "original"},
		"sliceString": []string{"original"},
	}

	clone := original.Clone().(Map)

	// These normalize -- the fallback cannot reconstruct an arbitrary named type.
	require.IsType(t, []any{}, clone["namedSlice"])
	require.IsType(t, map[string]any{}, clone["namedMap"])
	require.IsType(t, map[string]any{}, clone["mapString"])
	require.IsType(t, []any{}, clone["sliceString"])

	// ...but they must still be independent of the source.
	clone["namedSlice"].([]any)[0].(map[string]any)["nested"] = "MUTATED"
	clone["namedMap"].(map[string]any)["nested"] = "MUTATED"

	assert.Equal(t, "original", original["namedSlice"].(namedSlice)[0].(map[string]any)["nested"])
	assert.Equal(t, "original", original["namedMap"].(namedMap)["nested"])
}

// TestCloneChild_Passthrough guards the Nil{} trap: NewValue falls back to Nil{} for types it does
// not know, so cloning must pass such values through by assignment rather than round-tripping.
func TestCloneChild_Passthrough(t *testing.T) {

	type custom struct{ Name string }

	channel := make(chan int)
	pointer := &custom{Name: "kept"}

	original := Map{
		"struct":  custom{Name: "kept"},
		"chan":    channel,
		"pointer": pointer,
		"func":    TestCloneChild_Passthrough,
		"nil":     nil,
	}

	clone := original.Clone().(Map)

	assert.Equal(t, custom{Name: "kept"}, clone["struct"])
	assert.Equal(t, channel, clone["chan"])
	assert.Same(t, pointer, clone["pointer"])
	assert.IsType(t, TestCloneChild_Passthrough, clone["func"])
	assert.Nil(t, clone["nil"])
}

// TestCloneChild_ScalarTypes confirms cloning does not mangle number types -- an int64 must not
// come back as a float64, the way a JSON round-trip would leave it.
func TestCloneChild_ScalarTypes(t *testing.T) {

	now := time.Now()

	original := Map{
		"int64":   int64(1754750000000),
		"int":     42,
		"int32":   int32(7),
		"float32": float32(1.5),
		"float64": 1.5,
		"bool":    true,
		"string":  "text",
		"time":    now,
	}

	clone := original.Clone().(Map)

	assert.Equal(t, int64(1754750000000), clone["int64"])
	assert.Equal(t, 42, clone["int"])
	assert.Equal(t, int32(7), clone["int32"])
	assert.Equal(t, float32(1.5), clone["float32"])
	assert.Equal(t, 1.5, clone["float64"])
	assert.Equal(t, true, clone["bool"])
	assert.Equal(t, "text", clone["string"])
	assert.Equal(t, now, clone["time"])
}

/******************************************
 * Edge cases
 ******************************************/

// TestClone_EmptyAndNilContainers covers the zero-iteration paths through cloneMap and cloneSlice.
func TestClone_EmptyAndNilContainers(t *testing.T) {

	assert.Equal(t, Map{}, Map{}.Clone())
	assert.Equal(t, Slice{}, Slice{}.Clone())
	assert.Equal(t, Map{}, Map(nil).Clone())
	assert.Equal(t, Slice{}, Slice(nil).Clone())

	// A nil container nested inside a live one.
	original := Map{
		"nilMap":   map[string]any(nil),
		"nilSlice": []any(nil),
	}

	clone := original.Clone().(Map)
	assert.Equal(t, map[string]any{}, clone["nilMap"])
	assert.Equal(t, []any{}, clone["nilSlice"])
}

// TestClone_DeeplyNested confirms the recursion handles depth well past anything a real document
// carries, without relying on the fuzzer to find it.
func TestClone_DeeplyNested(t *testing.T) {

	const depth = 2_000

	original := map[string]any{"leaf": "original"}
	for range depth {
		original = map[string]any{"child": original}
	}

	clone := Map(original).Clone().(Map)

	// Walk to the bottom of the clone and mutate the leaf.
	cursor := map[string]any(clone)
	for range depth {
		cursor = cursor["child"].(map[string]any)
	}
	cursor["leaf"] = "MUTATED"

	// The original's leaf must be untouched.
	cursor = original
	for range depth {
		cursor = cursor["child"].(map[string]any)
	}
	assert.Equal(t, "original", cursor["leaf"])
}

// TestClone_RepeatedReferenceIsSplit documents a deliberate consequence: when the same container
// appears twice in one document, the clone gets two independent copies. JSON-LD from the wire is a
// tree, so this costs nothing in practice -- but it is behavior, so it is pinned.
func TestClone_RepeatedReferenceIsSplit(t *testing.T) {

	shared := map[string]any{"nested": "original"}
	original := Map{"first": shared, "second": shared}

	clone := original.Clone().(Map)
	clone["first"].(map[string]any)["nested"] = "MUTATED"

	assert.Equal(t, "MUTATED", clone["first"].(map[string]any)["nested"])
	assert.Equal(t, "original", clone["second"].(map[string]any)["nested"], "copies are independent of each other")
	assert.Equal(t, "original", shared["nested"], "and of the source")
}
