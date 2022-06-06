// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package maps

// ToSlice applies the function f to each element of the map and returns a new slice with the results.
func ToSlice[M ~map[K]V, Z any, K comparable, V any](m M, fn func(K, V) Z) []Z {
	r := make([]Z, 0, len(m))

	for k, v := range m {
		r = append(r, fn(k, v))
	}

	return r
}

// Map applies the function f to each element of the map and returns a new map with the results.
func Map[M ~map[K]V, K comparable, V any, K1 comparable, V1 any](m M, fn func(K, V) (K1, V1)) map[K1]V1 {
	r := make(map[K1]V1, len(m))

	for k, v := range m {
		k1, v1 := fn(k, v)
		r[k1] = v1
	}

	return r
}

// Keys returns the keys of the map m.
// The keys will be in an indeterminate order.
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))

	for k := range m {
		r = append(r, k)
	}

	return r
}

// KeysFunc applies the function f to each key of the map m and returns a new slice with the results.
// The keys will be in an indeterminate order.
func KeysFunc[M ~map[K]V, K comparable, V any, K1 any](m M, fn func(K) K1) []K1 {
	r := make([]K1, 0, len(m))

	for k := range m {
		r = append(r, fn(k))
	}

	return r
}

// ValuesFunc applies the function f to each value of the map m and returns a new slice with the results.
// The values will be in an indeterminate order.
func ValuesFunc[M ~map[K]V, K comparable, V any, V1 any](m M, fn func(V) V1) []V1 {
	r := make([]V1, 0, len(m))

	for _, v := range m {
		r = append(r, fn(v))
	}

	return r
}

// Contains reports whether m keys contains all the elements of s.
func Contains[M ~map[K]struct{}, K comparable](m M, s []K) bool {
	for _, v := range s {
		if _, ok := m[v]; !ok {
			return false
		}
	}

	return true
}

// Filter returns a map containing all the elements of m that satisfy f.
func Filter[M ~map[K]V, K comparable, V any](m M, f func(K, V) bool) map[K]V {
	r := make(map[K]V, len(m))

	for k, v := range m {
		if f(k, v) {
			r[k] = v
		}
	}

	// No reason to return empty map if there are no elements.
	if len(r) == 0 {
		return nil
	}

	return r
}

// FilterInPlace applies the function f to each element of the map and returns an old map with the filtered results.
func FilterInPlace[M ~map[K]V, K comparable, V any](m M, f func(K, V) bool) M {
	for k, v := range m {
		if !f(k, v) {
			delete(m, k)
		}
	}

	// No reason to return empty map if there are no elements.
	if len(m) == 0 {
		return nil
	}

	return m
}
