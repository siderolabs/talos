// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package algorithm provides generic algorithms for working with various data structures.
package algorithm

// Deduplicate takes a slice of any type T and a function that extracts
// a key K from T. It returns a slice with duplicates (by key) removed,
// keeping the first occurrence.
func Deduplicate[T any, K comparable](items []T, keyFunc func(T) K) []T {
	var (
		result []T
		seen   = make(map[K]bool)
	)

	for _, item := range items {
		if k := keyFunc(item); !seen[k] {
			seen[k] = true

			result = append(result, item)
		}
	}

	return result
}
