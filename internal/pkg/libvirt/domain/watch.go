// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"context"
	"net"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
)

// Watch subscribes to QEMU domain lifecycle changes on a dedicated connection.
// The returned channel is a coalescing wake-up signal (not an event log): it
// closes when ctx is canceled or the connection is lost. Callers should rescan
// the domain inventory on each signal and establish a new watch after closure.
// A successful return means registration has been acknowledged by libvirt.
func (c *Connector) Watch(ctx context.Context) (<-chan struct{}, error) {
	startupCtx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(startupCtx, "unix", c.socket)
	if err != nil {
		return nil, err
	}

	stopStartupClose := context.AfterFunc(startupCtx, func() { closeTransport(conn) })
	defer stopStartupClose()

	return c.WatchConn(ctx, conn)
}

// WatchConn is Watch over an already-connected transport. It takes ownership
// of conn even if the handshake or registration fails.
func (c *Connector) WatchConn(ctx context.Context, conn net.Conn) (<-chan struct{}, error) {
	stopClose := context.AfterFunc(ctx, func() { closeTransport(conn) })

	rpc := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))
	if err := rpc.ConnectToURI(libvirt.ConnectURI(c.uri)); err != nil {
		stopClose()
		closeTransport(conn)

		return nil, err
	}

	events, err := rpc.LifecycleEvents(ctx)
	if err != nil {
		stopClose()
		closeTransport(conn)

		return nil, err
	}

	changes := make(chan struct{}, 1)
	go func() {
		defer close(changes)
		defer closeTransport(conn)
		defer stopClose()

		for {
			select {
			case _, ok := <-events:
				if !ok {
					return
				}

				select {
				case changes <- struct{}{}:
				default:
				}
			case <-ctx.Done():
				return
			case <-rpc.Disconnected():
				return
			}
		}
	}()

	return changes, nil
}
