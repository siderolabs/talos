// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

func TestAPIPortAllocator(t *testing.T) {
	const count = 100

	allocator := qemu.APIPortAllocatorForTest{}
	ports := make(chan int, count)
	errs := make(chan error, count)

	var wg sync.WaitGroup

	for range count {
		wg.Go(func() {
			addr, err := allocator.Allocate(t.Context(), "127.0.0.1")
			if err != nil {
				errs <- err

				return
			}

			ports <- addr.Port
		})
	}

	wg.Wait()
	close(ports)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	uniquePorts := map[int]struct{}{}
	for port := range ports {
		_, exists := uniquePorts[port]
		require.False(t, exists, "allocated duplicate port %d", port)

		uniquePorts[port] = struct{}{}
	}

	require.Len(t, uniquePorts, count)
}
