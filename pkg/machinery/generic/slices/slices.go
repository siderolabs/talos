// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package slices

// Map applies the function f to each element of the slice and returns a new slice with the results.
func Map[S ~[]V, V any, R any](slc S, fn func(V) R) []R {
	r := make([]R, 0, len(slc))

	for _, v := range slc {
		r = append(r, fn(v))
	}

	return r
}

// ToMap converts a slice to a map.
func ToMap[S ~[]V, V any, R any, K comparable](slc S, fn func(V) (K, R)) map[K]R {
	r := make(map[K]R, len(slc))

	for _, v := range slc {
		key, val := fn(v)
		r[key] = val
	}

	return r
}

// ToSet converts a slice to a set.
func ToSet[K comparable](s []K) map[K]struct{} {
	r := make(map[K]struct{}, len(s))

	for _, v := range s {
		r[v] = struct{}{}
	}

	return r
}

// ToSetFunc converts a slice to a set using the function f.
func ToSetFunc[V any, K comparable](s []V, f func(V) K) map[K]struct{} {
	r := make(map[K]struct{}, len(s))
	for _, v := range s {
		r[f(v)] = struct{}{}
	}

	return r
}

// IndexFunc returns the first index i satisfying f(s[i]),
// or -1 if none do.
func IndexFunc[V any](s []V, f func(V) bool) int {
	for i, v := range s {
		if f(v) {
			return i
		}
	}

	return -1
}

// Contains reports whether v is present in s.
func Contains[K comparable](s []K, f func(K) bool) bool {
	return IndexFunc(s, f) >= 0
}

// IsSubset reports whether s is a subset of t.
func IsSubset[K comparable](set, subset []K) bool {
	inputMap := ToSet(set)

	for _, v := range subset {
		if _, found := inputMap[v]; !found {
			return false
		}
	}

	return true
}
