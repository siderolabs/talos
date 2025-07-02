// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package filehash implements a specialized file watcher that detects changes in pseudo-files like /proc/modules.
package filehash

import (
	"crypto/sha256"
	"io"
	"os"
	"time"
)

// Watcher monitors a file for changes by comparing its hash every second.
type Watcher struct {
	filepath string
	lastHash [32]byte
	quit     chan struct{}
}

// NewWatcher creates a new file watcher for the specified filepath.
func NewWatcher(filepath string) (*Watcher, error) {
	return &Watcher{
		filepath: filepath,
		quit:     make(chan struct{}),
	}, nil
}

// Close stops the watcher and releases resources.
func (w *Watcher) Close() error {
	close(w.quit)

	return nil
}

// Run polls the file every second and emits the path if the hash changes.
func (w *Watcher) Run() (<-chan string, <-chan error) {
	eventCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		defer close(eventCh)
		defer close(errCh)

		for {
			select {
			case <-w.quit:
				return
			case <-ticker.C:
				hash, err := hashFile(w.filepath)
				if err != nil {
					errCh <- err

					continue
				}

				if hash != w.lastHash {
					w.lastHash = hash
					eventCh <- w.filepath
				}
			}
		}
	}()

	return eventCh, errCh
}

func hashFile(path string) ([32]byte, error) {
	var zero [32]byte

	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close() //nolint:errcheck

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return zero, err
	}

	var sum [32]byte
	copy(sum[:], h.Sum(nil))

	return sum, nil
}
