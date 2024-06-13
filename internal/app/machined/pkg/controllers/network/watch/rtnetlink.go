// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package watch

import (
	"fmt"
	"sync"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/mdlayher/netlink"
)

type rtnetlinkWatcher struct {
	wg   sync.WaitGroup
	conn *rtnetlink.Conn
}

// NewRtNetlink starts rtnetlink watch over specified groups.
func NewRtNetlink(trigger Trigger, groups uint32) (Watcher, error) {
	watcher := &rtnetlinkWatcher{}

	var err error

	watcher.conn, err = rtnetlink.Dial(&netlink.Config{
		Groups: groups,
	})
	if err != nil {
		return nil, fmt.Errorf("error dialing watch socket: %w", err)
	}

	watcher.wg.Add(1)

	go func() {
		defer watcher.wg.Done()

		for {
			_, _, watchErr := watcher.conn.Receive()
			if watchErr != nil {
				return
			}

			trigger.QueueReconcile()
		}
	}()

	return watcher, nil
}

func (watcher *rtnetlinkWatcher) Done() {
	watcher.conn.Close() //nolint:errcheck

	watcher.wg.Wait()
}
