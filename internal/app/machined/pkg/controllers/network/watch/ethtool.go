// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package watch

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

type ethtoolWatcher struct {
	wg   sync.WaitGroup
	conn *netlink.Conn
}

// NewEthtool starts ethtool watch.
func NewEthtool(trigger Trigger) (Watcher, error) {
	watcher := &ethtoolWatcher{}

	var err error

	// the raw netlink connection is used for receiving, as the messages are never decoded,
	// while the generic netlink wrapper is only used to resolve the multicast group
	watcher.conn, err = netlink.Dial(unix.NETLINK_GENERIC, nil)
	if err != nil {
		return nil, fmt.Errorf("error dialing ethtool watch socket: %w", err)
	}

	monitorID, err := ethtoolMonitorGroup(genetlink.NewConn(watcher.conn))
	if err != nil {
		watcher.conn.Close() //nolint:errcheck

		return nil, err
	}

	if err = watcher.conn.JoinGroup(monitorID); err != nil {
		watcher.conn.Close() //nolint:errcheck

		return nil, fmt.Errorf("error joing multicast group for ethtool: %w", err)
	}

	watcher.wg.Go(func() {
		watch(watcher.conn, trigger)
	})

	return watcher, nil
}

func ethtoolMonitorGroup(conn *genetlink.Conn) (uint32, error) {
	ethFamily, err := conn.GetFamily(unix.ETHTOOL_GENL_NAME)
	if err != nil {
		return 0, fmt.Errorf("error getting family information for ethtool: %w", err)
	}

	for _, g := range ethFamily.Groups {
		if g.Name == unix.ETHTOOL_MCGRP_MONITOR_NAME {
			return g.ID, nil
		}
	}

	return 0, errors.New("could not find monitor multicast group ID for ethtool")
}

func (watcher *ethtoolWatcher) Done() {
	watcher.conn.Close() //nolint:errcheck

	watcher.wg.Wait()
}
