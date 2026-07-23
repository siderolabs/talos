// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"errors"
	"net"
	"sync"
)

type apiPortAllocator struct {
	mu        sync.Mutex
	allocated map[int]struct{}
}

func (allocator *apiPortAllocator) allocate(ctx context.Context, host string) (*net.TCPAddr, error) {
	allocator.mu.Lock()
	defer allocator.mu.Unlock()

	if allocator.allocated == nil {
		allocator.allocated = make(map[int]struct{})
	}

	var listeners []net.Listener

	closeListeners := func() error {
		var closeErr error

		for _, listener := range listeners {
			closeErr = errors.Join(closeErr, listener.Close())
		}

		return closeErr
	}

	for {
		listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", net.JoinHostPort(host, "0"))
		if err != nil {
			return nil, errors.Join(err, closeListeners())
		}

		listeners = append(listeners, listener)

		addr := listener.Addr().(*net.TCPAddr)
		if _, exists := allocator.allocated[addr.Port]; exists {
			continue
		}

		if err = closeListeners(); err != nil {
			return nil, err
		}

		allocator.allocated[addr.Port] = struct{}{}

		return addr, nil
	}
}
