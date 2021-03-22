// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package follow provides Reader which follows file updates and turns it into a stream.
package follow

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Reader implements io.ReadCloser over regular file following file contents.
//
// This makes file similar to the stream in semantics.
type Reader struct {
	source *os.File

	ctx       context.Context
	ctxCancel context.CancelFunc

	notifyCh chan error

	mu            sync.Mutex
	closed        bool
	notifyStarted bool
}

// NewReader wraps io.File as follow.Reader.
func NewReader(readCtx context.Context, source *os.File) *Reader {
	ctx, ctxCancel := context.WithCancel(readCtx)

	return &Reader{
		source: source,

		notifyCh: make(chan error, 1),

		ctx:       ctx,
		ctxCancel: ctxCancel,
	}
}

// Read implements io.Reader interface.
func (r *Reader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	if r.closed {
		err = io.ErrClosedPipe
		r.mu.Unlock()

		return
	}

	if !r.notifyStarted {
		r.startNotify()
	}
	r.mu.Unlock()

	select {
	case <-r.ctx.Done():
		err = io.EOF

		return
	default:
	}

	for {
		n, err = r.source.Read(p)
		if err == nil || err != io.EOF {
			return
		}

		select {
		case <-r.ctx.Done():
			err = io.EOF

			return
		case err = <-r.notifyCh:
			if err != nil {
				return
			}
		}
	}
}

// Close implements io.Closer interface.
func (r *Reader) Close() error {
	r.mu.Lock()

	if r.closed {
		r.mu.Unlock()

		return nil
	}

	r.closed = true

	r.mu.Unlock()

	r.ctxCancel()

	return r.source.Close()
}

func (r *Reader) startNotify() {
	r.notifyStarted = true

	go r.notify()
}

//nolint:gocyclo
func (r *Reader) notify() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		select {
		case r.notifyCh <- fmt.Errorf("failed to watch: %w", err):
		case <-r.ctx.Done():
		}

		return
	}

	//nolint:errcheck
	defer watcher.Close()

	filename := r.source.Name()

	if err = watcher.Add(filepath.Dir(filename)); err != nil {
		select {
		case r.notifyCh <- fmt.Errorf("failed to add dir watch: %w", err):
		case <-r.ctx.Done():
		}

		return
	}

	for {
		select {
		case <-r.ctx.Done():
			return
		case event := <-watcher.Events:
			if event.Name != filename {
				// ignore events for other files
				continue
			}

			switch event.Op { //nolint:exhaustive
			case fsnotify.Write:
				// non-blocking send, we need to keep processing fsnotify events
				// at least signal message is in r.notifyCh which will allow Read to wake up
				select {
				case r.notifyCh <- nil:
				default:
				}
			case fsnotify.Remove:
				select {
				case r.notifyCh <- fmt.Errorf("file was removed while watching"):
				case <-r.ctx.Done():
				}

				return
			}
		case err := <-watcher.Errors:
			select {
			case r.notifyCh <- fmt.Errorf("failed to watch: %w", err):
			case <-r.ctx.Done():
			}

			return
		}
	}
}
