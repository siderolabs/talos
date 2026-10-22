// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package watch provides netlink watchers via multicast groups.
package watch

import (
	"errors"
	"os"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

// Watcher interface allows to stop watching.
type Watcher interface {
	Done()
}

// Trigger is used by watcher to trigger reconcile loops.
type Trigger interface {
	QueueReconcile()
}

// receiver is the part of *netlink.Conn used by the watch loop.
type receiver interface {
	Receive() ([]netlink.Message, error)
}

// watch receives multicast notifications from the connection until it is closed, queueing a reconcile on each of them.
//
// The contents of the notifications are never inspected: any activity on the subscribed multicast groups is just a hint
// that the kernel state might have changed, and the controllers re-list the full state on reconcile.
//
// For the same reason the only error which stops the loop is the connection being closed (via Done()).
// In particular, ENOBUFS is reported once by the kernel after the socket receive buffer overflowed and some notifications
// were dropped, but the socket is fully usable afterwards; the dropped notifications are compensated for by queueing
// a reconcile. Any other error (e.g. a malformed datagram) doesn't make the socket unusable either, as the offending
// datagram has already been consumed.
func watch(conn receiver, trigger Trigger) {
	for {
		if _, err := conn.Receive(); err != nil && isClosed(err) {
			return
		}

		trigger.QueueReconcile()
	}
}

// isClosed returns true if the error indicates that the netlink socket was closed.
//
// EBADF is returned for any operation attempted after Close(), while os.ErrClosed is returned by the runtime poller
// if Close() raced with the receive in progress.
func isClosed(err error) bool {
	return errors.Is(err, unix.EBADF) || errors.Is(err, os.ErrClosed)
}
