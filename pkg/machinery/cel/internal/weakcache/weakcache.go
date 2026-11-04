// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package weakcache implements a cache of values derived from objects, which doesn't keep the objects alive.
package weakcache

import (
	"runtime"
	"sync"
	"weak"
)

// Cache maps an object (by its pointer) and a key to a value.
//
// The object is referenced weakly: once it is garbage collected, its values are removed from the cache.
// The value must not reference the object, otherwise the object is never collected.
//
// Cache is safe for concurrent use. The zero value is ready to use.
// Cache must not be copied after the first use, and it should be long-lived: entries reference it until they are removed.
type Cache[T any, K comparable, V any] struct {
	m sync.Map // map[entryKey[T, K]]V
}

type entryKey[T any, K comparable] struct {
	obj weak.Pointer[T]
	key K
}

// Get returns the value for the object and the key.
//
// Get doesn't allocate.
func (c *Cache[T, K, V]) Get(obj *T, key K) (V, bool) {
	v, ok := c.m.Load(entryKey[T, K]{obj: weak.Make(obj), key: key})
	if !ok {
		var zero V

		return zero, false
	}

	return v.(V), true //nolint:forcetypeassert
}

// Store stores the value for the object and the key, unless there is a value already.
//
// Store returns the value in the cache: if multiple goroutines store a value concurrently, all of them get the same value.
func (c *Cache[T, K, V]) Store(obj *T, key K, value V) V {
	if obj == nil {
		panic("weakcache: nil object")
	}

	k := entryKey[T, K]{obj: weak.Make(obj), key: key}

	if actual, loaded := c.m.LoadOrStore(k, value); loaded {
		return actual.(V) //nolint:forcetypeassert
	}

	runtime.AddCleanup(obj, c.delete, k)

	return value
}

// Len returns the number of values in the cache.
func (c *Cache[T, K, V]) Len() int {
	n := 0

	c.m.Range(func(any, any) bool {
		n++

		return true
	})

	return n
}

func (c *Cache[T, K, V]) delete(k entryKey[T, K]) {
	c.m.Delete(k)
}
