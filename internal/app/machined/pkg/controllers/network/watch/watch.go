// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package watch provides netlink watchers via multicast groups.
package watch

// Watcher interface allows to stop watching.
type Watcher interface {
	Done()
}

// Trigger is used by watcher to trigger reconcile loops.
type Trigger interface {
	QueueReconcile()
}
