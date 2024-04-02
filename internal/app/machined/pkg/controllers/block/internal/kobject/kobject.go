// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kobject implements Linux kernel kobject uvent watcher.
package kobject

import (
	"fmt"
	"sync"

	"github.com/mdlayher/kobject"
	"go.uber.org/zap"
)

const readBufferSize = 64 * 1024 * 1024

// Event is exported.
type Event = kobject.Event

// Re-export action constants.
const (
	ActionAdd     = kobject.Add
	ActionRemove  = kobject.Remove
	ActionChange  = kobject.Change
	ActionMove    = kobject.Move
	ActionOnline  = kobject.Online
	ActionOffline = kobject.Offline
	ActionBind    = kobject.Bind
	ActionUnbind  = kobject.Unbind
)

// Watcher is a kobject uvent watcher.
type Watcher struct {
	wg sync.WaitGroup

	cli *kobject.Client
}

// NewWatcher creates a new kobject watcher.
func NewWatcher() (*Watcher, error) {
	cli, err := kobject.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create kobject client: %w", err)
	}

	if err = cli.SetReadBuffer(readBufferSize); err != nil {
		return nil, err
	}

	return &Watcher{
		cli: cli,
	}, nil
}

// Close the watcher.
func (w *Watcher) Close() error {
	if err := w.cli.Close(); err != nil {
		return err
	}

	w.wg.Wait()

	return nil
}

// Run the watcher, returns the channel of events.
func (w *Watcher) Run(logger *zap.Logger) <-chan *Event {
	ch := make(chan *kobject.Event, 128)

	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		defer close(ch)

		for {
			ev, err := w.cli.Receive()
			if err != nil {
				if err.Error() != "use of closed file" { // unfortunately not an exported error, just errors.New()
					logger.Error("failed to receive kobject event", zap.Error(err))
				}

				return
			}

			ch <- ev
		}
	}()

	return ch
}
