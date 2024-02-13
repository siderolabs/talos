// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package watch

import (
	"errors"
	"fmt"
	"sync"

	"github.com/mdlayher/genetlink"
	"golang.org/x/sys/unix"
)

type ethtoolWatcher struct {
	wg   sync.WaitGroup
	conn *genetlink.Conn
}

// NewEthtool starts ethtool watch.
func NewEthtool(trigger Trigger) (Watcher, error) {
	watcher := &ethtoolWatcher{}

	var err error

	watcher.conn, err = genetlink.Dial(nil)
	if err != nil {
		return nil, fmt.Errorf("error dialing ethtool watch socket: %w", err)
	}

	ethFamily, err := watcher.conn.GetFamily(unix.ETHTOOL_GENL_NAME)
	if err != nil {
		return nil, fmt.Errorf("error getting family information for ethtool: %w", err)
	}

	var monitorID uint32

	for _, g := range ethFamily.Groups {
		if g.Name == unix.ETHTOOL_MCGRP_MONITOR_NAME {
			monitorID = g.ID

			break
		}
	}

	if monitorID == 0 {
		return nil, errors.New("could not find monitor multicast group ID for ethtool")
	}

	if err = watcher.conn.JoinGroup(monitorID); err != nil {
		return nil, fmt.Errorf("error joing multicast group for ethtool: %w", err)
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

func (watcher *ethtoolWatcher) Done() {
	watcher.conn.Close() //nolint:errcheck

	watcher.wg.Wait()
}
