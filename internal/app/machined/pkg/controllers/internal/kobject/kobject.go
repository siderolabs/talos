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

type receiver interface {
	Receive() (*kobject.Event, error)
	Close() error
}

// Watcher is a kobject uevent watcher.
type Watcher struct {
	wg     sync.WaitGroup
	cli    receiver
	logger *zap.Logger
	errCh  chan error
}

// NewWatcher creates a new kobject watcher.
// subsystem is used to filter events by subsystem.
func NewWatcher(logger *zap.Logger) (*Watcher, error) {
	cli, err := kobject.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create kobject client: %w", err)
	}

	if err = cli.SetReadBuffer(readBufferSize); err != nil {
		return nil, err
	}

	return NewWatcherFromReceiver(cli, logger), nil
}

// NewWatcherFromReceiver creates a new kobject watcher from a custom receiver.
func NewWatcherFromReceiver(r receiver, logger *zap.Logger) *Watcher {
	return &Watcher{
		cli:    r,
		logger: logger,
		errCh:  make(chan error, 1),
	}
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
// subsystem is used to filter events by subsystem.
func (w *Watcher) Run(subsystem string) <-chan *Event {
	ch := make(chan *kobject.Event, 128)

	w.wg.Go(func() {
		defer close(ch)

		for {
			ev, err := w.cli.Receive()
			if err != nil {
				if err.Error() != "use of closed file" { // unfortunately not an exported error, just errors.New()
					w.logger.Error("failed to receive kobject event", zap.Error(err))

					select {
					case w.errCh <- err:
					default:
					}
				}

				return
			}

			if ev.Subsystem != subsystem {
				continue
			}

			ch <- ev
		}
	})

	return ch
}

// ErrCh returns a channel that receives the first fatal error from the watcher goroutine.
func (w *Watcher) ErrCh() <-chan error {
	return w.errCh
}
