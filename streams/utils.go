package streams

import (
	"iter"
)

// joinIterators combines multiple rangeFunc iterators into a single iterator.
func joinIterators[T any](iterators ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, iterator := range iterators {
			for value := range iterator {
				if !yield(value) {
					return
				}
			}
		}
	}
}
