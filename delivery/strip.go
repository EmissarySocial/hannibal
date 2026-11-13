package delivery

import (
	"reflect"
	"slices"

	"github.com/benpate/hannibal/vocab"
)

// StripBlindRecipients returns a deep copy of a document with `bto` and `bcc` removed at every
// level of nesting. The original document is not changed.
func StripBlindRecipients(document map[string]any) map[string]any {
	return stripMap(reflect.ValueOf(document))
}

// isBlindProperty returns TRUE if a property names recipients that no other recipient may see
func isBlindProperty(name string) bool {
	return (name == vocab.PropertyBTo) || (name == vocab.PropertyBCC)
}

// stripValue returns a deep copy of any JSON-like value, without blind recipients
func stripValue(value any) any {

	// The common leaf and container types need no reflection
	switch typed := value.(type) {

	case nil, string, bool, float64, float32, int, int64:
		return value

	case []string:
		return slices.Clone(typed)

	case []byte:
		return slices.Clone(typed)
	}

	// RULE: Every other map and slice is walked by its kind, so that a named type from another
	// package (a database driver's array, for example) cannot carry a blind recipient through
	switch reflected := reflect.ValueOf(value); reflected.Kind() {

	case reflect.Map:
		return stripAnyMap(value, reflected)

	case reflect.Slice:
		return stripSlice(reflected)

	default:
		return value
	}
}

// stripAnyMap copies a map that has string keys, and returns any other map as it is
func stripAnyMap(value any, reflected reflect.Value) any {

	// A map without string keys is not a JSON object, and has no property names to check
	if reflected.Type().Key().Kind() != reflect.String {
		return value
	}

	return stripMap(reflected)
}

// stripMap returns a deep copy of a map with string keys, without blind recipients
func stripMap(reflected reflect.Value) map[string]any {

	result := make(map[string]any, reflected.Len())

	for iterator := reflected.MapRange(); iterator.Next(); {

		name := iterator.Key().String()

		if isBlindProperty(name) {
			continue
		}

		result[name] = stripValue(iterator.Value().Interface())
	}

	return result
}

// stripSlice returns a deep copy of a slice, without blind recipients. A slice of key/value
// pairs, which is how some decoders represent an ordered document, is returned as a map.
func stripSlice(reflected reflect.Value) any {

	if isKeyValueSlice(reflected.Type()) {
		return stripKeyValues(reflected)
	}

	result := make([]any, reflected.Len())

	for index := range reflected.Len() {
		result[index] = stripValue(reflected.Index(index).Interface())
	}

	return result
}

// isKeyValueSlice returns TRUE if a slice holds structs made of a string `Key` and a `Value`
func isKeyValueSlice(sliceType reflect.Type) bool {

	element := sliceType.Elem()

	if element.Kind() != reflect.Struct {
		return false
	}

	if element.NumField() != 2 {
		return false
	}

	if key, hasKey := element.FieldByName("Key"); !hasKey || key.Type.Kind() != reflect.String {
		return false
	}

	_, hasValue := element.FieldByName("Value")
	return hasValue
}

// stripKeyValues returns an ordered document as a map, without blind recipients
func stripKeyValues(reflected reflect.Value) map[string]any {

	result := make(map[string]any, reflected.Len())

	for index := range reflected.Len() {

		pair := reflected.Index(index)
		name := pair.FieldByName("Key").String()

		if isBlindProperty(name) {
			continue
		}

		result[name] = stripValue(pair.FieldByName("Value").Interface())
	}

	return result
}
