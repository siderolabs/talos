// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package maps contains the generic functions for maps.
package maps

// NOTE(DmitriyMV): I tried to implement this generic functions to be as perfomant as possible.
// However, I couldn't find a way to do it, since Go (1.18 at the time of writing) cannot inline closures if (generic)
// function, which accepts the closure, was not inlined itself.
// And inlining budget of 80 is quite small, since most of it is going towards closure call.
// Somewhat relevant issue: https://github.com/golang/go/issues/41988

// ToSlice applies the function fn to each element of the map and returns a new slice with the results.
func ToSlice[K comparable, V any, R any](m map[K]V, fn func(K, V) R) []R {
	if len(m) == 0 {
		return nil
	}

	r := make([]R, 0, len(m))

	for k, v := range m {
		r = append(r, fn(k, v))
	}

	return r
}

// Map applies the function fn to each element of the map and returns a new map with the results.
func Map[K comparable, V any, K1 comparable, V1 any](m map[K]V, fn func(K, V) (K1, V1)) map[K1]V1 {
	if len(m) == 0 {
		return nil
	}

	r := make(map[K1]V1, len(m))

	for k, v := range m {
		k1, v1 := fn(k, v)
		r[k1] = v1
	}

	return r
}

// Keys returns the keys of the map m.
// The keys will be in an indeterminate order.
func Keys[K comparable, V any](m map[K]V) []K {
	if len(m) == 0 {
		return nil
	}

	r := make([]K, 0, len(m))

	for k := range m {
		r = append(r, k)
	}

	return r
}

// KeysFunc applies the function fn to each key of the map m and returns a new slice with the results.
// The keys will be in an indeterminate order.
func KeysFunc[K comparable, V, R any](m map[K]V, fn func(K) R) []R {
	if len(m) == 0 {
		return nil
	}

	r := make([]R, 0, len(m))

	for k := range m {
		r = append(r, fn(k))
	}

	return r
}

// Values returns the values of the map m.
// The values will be in an indeterminate order.
func Values[K comparable, V any](m map[K]V) []V {
	r := make([]V, 0, len(m))

	for _, v := range m {
		r = append(r, v)
	}

	return r
}

// ValuesFunc applies the function fn to each value of the map m and returns a new slice with the results.
// The values will be in an indeterminate order.
func ValuesFunc[K comparable, V, R any](m map[K]V, fn func(V) R) []R {
	if len(m) == 0 {
		return nil
	}

	r := make([]R, 0, len(m))

	for _, v := range m {
		r = append(r, fn(v))
	}

	return r
}

// Contains reports whether m keys contains all the elements of the slice slc.
func Contains[K comparable](m map[K]struct{}, slc []K) bool {
	for _, v := range slc {
		if _, ok := m[v]; !ok {
			return false
		}
	}

	return true
}

// Intersect returns a list of keys contained in both maps.
func Intersect[K comparable](maps ...map[K]struct{}) []K {
	var intersection []K

	if len(maps) == 0 {
		return intersection
	}

	for k := range maps[0] {
		containedInAll := true

		for _, m := range maps[1:] {
			if _, ok := m[k]; !ok {
				containedInAll = false

				break
			}
		}

		if containedInAll {
			intersection = append(intersection, k)
		}
	}

	return intersection
}

// Filter returns a map containing all the elements of m that satisfy fn.
func Filter[M ~map[K]V, K comparable, V any](m M, fn func(K, V) bool) M {
	// NOTE(DmitriyMV): We use type parameter M here to return exactly the same tyoe as the input map.
	if len(m) == 0 {
		return nil
	}

	r := make(M, len(m))

	for k, v := range m {
		if fn(k, v) {
			r[k] = v
		}
	}

	// No reason to return empty map if it's empty.
	if len(r) == 0 {
		return nil
	}

	return r
}

// FilterInPlace applies the function fn to each element of the map and returns an old map with the filtered results.
func FilterInPlace[M ~map[K]V, K comparable, V any](m M, fn func(K, V) bool) M {
	// We return original empty map instead of a nil map unlike Filter function.
	if len(m) == 0 {
		return m
	}

	// NOTE(DmitriyMV): We use type parameter M here to return exactly the same tyoe as the input map.
	for k, v := range m {
		if !fn(k, v) {
			delete(m, k)
		}
	}

	// We return original map even if we filtered everything out unlike Filter function.
	return m
}
