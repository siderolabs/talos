// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package follow provides Reader which follows file updates and turns it into a stream.
package follow

import (
	"context"
	"errors"
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

	//nolint:containedctx
	ctx       context.Context
	ctxCancel context.CancelFunc

	notifyCh chan error

	mu            sync.Mutex
	closed        bool
	notifyStarted bool
	total         int64 // Tracks the current file size
	total         int64 // Tracks the current file size
}

// NewReader wraps io.File as follow.Reader.
func NewReader(readCtx context.Context, source *os.File) *Reader {
	ctx, ctxCancel := context.WithCancel(readCtx)

	// Initialize with current file size
	info, err := source.Stat()
	var total int64
	if err == nil {
		total = info.Size()
	}

	// Initialize with current file size
	info, err := source.Stat()
	var total int64
	if err == nil {
		total = info.Size()
	}

	return &Reader{
		source: source,

		notifyCh: make(chan error, 1),

		ctx:       ctx,
		ctxCancel: ctxCancel,
		total:     total,
		total:     total,
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

	// Cancel context first to stop any ongoing operations
	r.ctxCancel()

	// Ensure we close the source file
	err := r.source.Close()

	// Give time for goroutines to clean up
	time.Sleep(100 * time.Millisecond)

	return err
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

	defer func() {
		err := watcher.Close()
		if err != nil {
			select {
			case r.notifyCh <- fmt.Errorf("failed to close watcher: %w", err):
			case <-r.ctx.Done():
			}
		}
	}()

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

			// Check for file size change before backing off
			info, statErr := r.source.Stat()
			if statErr == nil {
				newSize := info.Size()
				if newSize > r.total {
					r.total = newSize
					select {
					case r.notifyCh <- nil:
					default:
					}
					continue
				}
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
				case r.notifyCh <- errors.New("file was removed while watching"):
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
