// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package watch

import (
	"context"
	"fmt"
	"sync"

	"github.com/jsimonetti/rtnetlink"
	"github.com/mdlayher/netlink"
)

type rtnetlinkWatcher struct {
	wg     sync.WaitGroup
	cancel context.CancelFunc
	conn   *rtnetlink.Conn
}

// NewRtNetlink starts rtnetlink watch over specified groups.
func NewRtNetlink(ctx context.Context, watchCh chan<- struct{}, groups uint32) (Watcher, error) {
	watcher := &rtnetlinkWatcher{}

	ctx, watcher.cancel = context.WithCancel(ctx)

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

			select {
			case watchCh <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return watcher, nil
}

func (watcher *rtnetlinkWatcher) Done() {
	watcher.cancel()
	watcher.conn.Close() //nolint:errcheck

	watcher.wg.Wait()
}
