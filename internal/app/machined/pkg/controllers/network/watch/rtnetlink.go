// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package watch

import (
	"fmt"
	"sync"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

type rtnetlinkWatcher struct {
	wg   sync.WaitGroup
	conn *netlink.Conn
}

// NewRtNetlink starts rtnetlink watch over specified groups.
func NewRtNetlink(trigger Trigger, groups uint32, netlinkGroups ...uint32) (Watcher, error) {
	watcher := &rtnetlinkWatcher{}

	var err error

	// the raw netlink connection is used, as the messages are never decoded
	watcher.conn, err = netlink.Dial(unix.NETLINK_ROUTE, &netlink.Config{
		Groups: groups,
	})
	if err != nil {
		return nil, fmt.Errorf("error dialing watch socket: %w", err)
	}

	for _, group := range netlinkGroups {
		if err := watcher.conn.JoinGroup(group); err != nil {
			watcher.conn.Close() //nolint:errcheck

			return nil, fmt.Errorf("error joining group %d: %w", group, err)
		}
	}

	watcher.wg.Go(func() {
		watch(watcher.conn, trigger)
	})

	return watcher, nil
}

func (watcher *rtnetlinkWatcher) Done() {
	watcher.conn.Close() //nolint:errcheck

	watcher.wg.Wait()
}
