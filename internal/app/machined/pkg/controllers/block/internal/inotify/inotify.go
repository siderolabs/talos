// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package inotify implements a specialized inotify watcher for block devices.
package inotify

import (
	"errors"
	"os"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

type (
	watches struct {
		mu   sync.RWMutex
		wd   map[uint32]*watch // wd → watch
		path map[string]uint32 // pathname → wd
	}
	watch struct {
		wd    uint32 // Watch descriptor (as returned by the inotify_add_watch() syscall)
		flags uint32 // inotify flags of this watch (see inotify(7) for the list of valid flags)
		path  string // Watch path.
	}
)

func newWatches() *watches {
	return &watches{
		wd:   make(map[uint32]*watch),
		path: make(map[string]uint32),
	}
}

func (w *watches) remove(wd uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.wd[wd]; ok {
		delete(w.path, w.wd[wd].path)
	}

	delete(w.wd, wd)
}

func (w *watches) removePath(path string) (uint32, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	wd, ok := w.path[path]
	if !ok {
		return 0, false
	}

	delete(w.path, path)
	delete(w.wd, wd)

	return wd, true
}

func (w *watches) byWd(wd uint32) *watch {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.wd[wd]
}

func (w *watches) updatePath(path string, f func(*watch) (*watch, error)) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var existing *watch

	wd, ok := w.path[path]
	if ok {
		existing = w.wd[wd]
	}

	upd, err := f(existing)
	if err != nil {
		return err
	}

	if upd != nil {
		w.wd[upd.wd] = upd
		w.path[upd.path] = upd.wd

		if upd.wd != wd {
			delete(w.wd, wd)
		}
	}

	return nil
}

// Watcher implements inotify-based file watching.
type Watcher struct {
	wg          sync.WaitGroup
	fd          int
	inotifyFile *os.File
	watches     *watches
}

// NewWatcher creates a new inotify Watcher.
func NewWatcher() (*Watcher, error) {
	// Need to set nonblocking mode for SetDeadline to work, otherwise blocking
	// I/O operations won't terminate on close.
	fd, errno := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if fd == -1 {
		return nil, errno
	}

	return &Watcher{
		fd:          fd,
		inotifyFile: os.NewFile(uintptr(fd), ""),
		watches:     newWatches(),
	}, nil
}

// Close the inotify watcher.
func (w *Watcher) Close() error {
	// Causes any blocking reads to return with an error, provided the file
	// still supports deadline operations.
	err := w.inotifyFile.Close()
	if err != nil {
		return err
	}

	// Wait for goroutine to close
	w.wg.Wait()

	return nil
}

// Run the watcher, returns two channels for errors and events (paths changed).
//
//nolint:gocyclo
func (w *Watcher) Run() (<-chan string, <-chan error) {
	errCh := make(chan error, 1)
	eventCh := make(chan string, 128)

	w.wg.Add(1)

	var buf [unix.SizeofInotifyEvent * 4096]byte // Buffer for a maximum of 4096 raw events

	go func() {
		defer w.wg.Done()

		for {
			n, err := w.inotifyFile.Read(buf[:])

			switch {
			case errors.Is(err, os.ErrClosed):
				return
			case err != nil:
				errCh <- err

				return
			}

			if n < unix.SizeofInotifyEvent {
				errCh <- errors.New("short read from inotify")

				return
			}

			var offset uint32

			// We don't know how many events we just read into the buffer
			// While the offset points to at least one whole event...
			for offset <= uint32(n-unix.SizeofInotifyEvent) {
				var (
					// Point "raw" to the event in the buffer
					raw     = (*unix.InotifyEvent)(unsafe.Pointer(&buf[offset]))
					mask    = raw.Mask
					nameLen = raw.Len
				)

				if mask&unix.IN_Q_OVERFLOW != 0 {
					errCh <- errors.New("inotify queue overflow")

					return
				}

				// If the event happened to the watched directory or the watched file, the kernel
				// doesn't append the filename to the event, but we would like to always fill the
				// the "Name" field with a valid filename. We retrieve the path of the watch from
				// the "paths" map.
				watch := w.watches.byWd(uint32(raw.Wd))

				// inotify will automatically remove the watch on deletes; just need
				// to clean our state here.
				if watch != nil && mask&unix.IN_DELETE_SELF == unix.IN_DELETE_SELF {
					w.watches.remove(watch.wd)
				}

				var name string
				if watch != nil {
					name = watch.path
				}

				if nameLen > 0 {
					// Point "bytes" at the first byte of the filename
					bytes := (*[unix.PathMax]byte)(unsafe.Pointer(&buf[offset+unix.SizeofInotifyEvent]))[:nameLen:nameLen]
					// The filename is padded with NULL bytes. TrimRight() gets rid of those.
					name += "/" + strings.TrimRight(string(bytes[0:nameLen]), "\000")
				}

				// Send the events that are not ignored on the events channel
				if mask&unix.IN_IGNORED == 0 && mask&unix.IN_CLOSE_WRITE != 0 {
					eventCh <- name
				}

				// Move to the next event in the buffer
				offset += unix.SizeofInotifyEvent + nameLen
			}
		}
	}()

	return eventCh, errCh
}

// Add a watch to the inotify watcher.
func (w *Watcher) Add(name string) error {
	var flags uint32 = unix.IN_CLOSE_WRITE | unix.IN_DELETE_SELF

	return w.watches.updatePath(name, func(existing *watch) (*watch, error) {
		if existing != nil {
			flags |= existing.flags | unix.IN_MASK_ADD
		}

		wd, err := unix.InotifyAddWatch(w.fd, name, flags)
		if wd == -1 {
			return nil, err
		}

		if existing == nil {
			return &watch{
				wd:    uint32(wd),
				path:  name,
				flags: flags,
			}, nil
		}

		existing.wd = uint32(wd)
		existing.flags = flags

		return existing, nil
	})
}

// Remove a watch from the inotify watcher.
func (w *Watcher) Remove(name string) error {
	wd, ok := w.watches.removePath(name)
	if !ok {
		return nil
	}

	success, errno := unix.InotifyRmWatch(w.fd, wd)
	if success == -1 {
		// TODO: Perhaps it's not helpful to return an error here in every case;
		//       The only two possible errors are:
		//
		//       - EBADF, which happens when w.fd is not a valid file descriptor
		//         of any kind.
		//       - EINVAL, which is when fd is not an inotify descriptor or wd
		//         is not a valid watch descriptor. Watch descriptors are
		//         invalidated when they are removed explicitly or implicitly;
		//         explicitly by inotify_rm_watch, implicitly when the file they
		//         are watching is deleted.
		return errno
	}

	return nil
}
