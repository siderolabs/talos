// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package weakcache_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/cel/internal/weakcache"
)

type object struct {
	name string
}

func TestGetStore(t *testing.T) {
	t.Parallel()

	var cache weakcache.Cache[object, int, string]

	obj := &object{name: "a"}

	_, ok := cache.Get(obj, 1)
	assert.False(t, ok)

	assert.Equal(t, "a1", cache.Store(obj, 1, "a1"))

	v, ok := cache.Get(obj, 1)
	assert.True(t, ok)
	assert.Equal(t, "a1", v)

	// the first stored value wins
	assert.Equal(t, "a1", cache.Store(obj, 1, "other"))

	// the key is part of the cache key
	_, ok = cache.Get(obj, 2)
	assert.False(t, ok)

	// objects are identified by the pointer, not by the contents
	_, ok = cache.Get(&object{name: "a"}, 1)
	assert.False(t, ok)

	assert.Equal(t, 1, cache.Len())

	assert.Panics(t, func() {
		cache.Store(nil, 1, "nil")
	})

	runtime.KeepAlive(obj)
}

func TestStoreConcurrent(t *testing.T) {
	t.Parallel()

	var (
		cache weakcache.Cache[object, int, *int]
		wg    sync.WaitGroup
	)

	obj := &object{}

	var results [16]*int

	for i := range results {
		wg.Go(func() {
			results[i] = cache.Store(obj, 0, new(i))
		})
	}

	wg.Wait()

	for _, result := range results {
		assert.Same(t, results[0], result)
	}

	runtime.KeepAlive(obj)
}

func TestCleanup(t *testing.T) {
	t.Parallel()

	var cache weakcache.Cache[object, int, string]

	alive := &object{name: "alive"}

	cache.Store(alive, 0, "alive")

	func() {
		for i := range 10 {
			obj := &object{name: "dropped"}

			cache.Store(obj, 0, "zero")
			cache.Store(obj, i+1, "i")
		}
	}()

	assert.Equal(t, 1+10*2, cache.Len())

	// the values of the collected objects are removed
	assert.Eventually(t, func() bool {
		runtime.GC()

		return cache.Len() == 1
	}, 10*time.Second, 10*time.Millisecond)

	v, ok := cache.Get(alive, 0)
	assert.True(t, ok)
	assert.Equal(t, "alive", v)

	runtime.KeepAlive(alive)
}

func TestGetAllocs(t *testing.T) {
	var cache weakcache.Cache[object, *object, any]

	obj, key := &object{}, &object{}

	cache.Store(obj, key, "value")

	allocs := testing.AllocsPerRun(100, func() {
		_, ok := cache.Get(obj, key)
		require.True(t, ok)
	})

	assert.Zero(t, allocs)

	runtime.KeepAlive(obj)
}
