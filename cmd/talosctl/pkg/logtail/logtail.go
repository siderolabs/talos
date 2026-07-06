// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package logtail streams complete lines of a file with `tail -F`
// semantics: while following, it reopens the file when it is recreated
// (new inode) or truncated.
package logtail

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// pollInterval is how often a followed file is re-checked once EOF is hit.
const pollInterval = 500 * time.Millisecond

// EmitFunc receives one complete log line (trailing newline stripped).
// Returning false stops the tail.
type EmitFunc func(line []byte) bool

// waitForFile polls until the file at path can be opened, returning the open
// handle. It returns ctx.Err() if the context is canceled while waiting.
func waitForFile(ctx context.Context, path string) (*os.File, error) {
	for {
		if f, err := os.Open(path); err == nil {
			return f, nil
		}

		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Tail streams lines of the file at path to emit. With follow, it keeps
// polling for new output until ctx is done, reopening the file if it is
// recreated or truncated — i.e. `tail -F` behavior.
//
//nolint:gocyclo
func Tail(ctx context.Context, path string, follow bool, emit EmitFunc) {
	f, err := os.Open(path)
	if err != nil {
		if !follow {
			emit(fmt.Appendf(nil, "(console log unavailable: %v)", err))

			return
		}

		// tail -F: the file may not exist yet (e.g. right after cluster start),
		// so wait for it to appear instead of giving up.
		if f, err = waitForFile(ctx, path); err != nil {
			return
		}
	}

	defer func() { f.Close() }() //nolint:errcheck

	var (
		partial []byte
		offset  int64
	)

	buf := make([]byte, 8192)

	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			offset += int64(n)
			partial = append(partial, buf[:n]...)

			for {
				idx := bytes.IndexByte(partial, '\n')
				if idx < 0 {
					break
				}

				if !emit(bytes.TrimRight(partial[:idx], "\r")) {
					return
				}

				partial = append([]byte(nil), partial[idx+1:]...)
			}
		}

		switch {
		case readErr == io.EOF && !follow:
			if len(partial) > 0 {
				emit(partial)
			}

			return
		case readErr == io.EOF:
			select {
			case <-time.After(pollInterval):
			case <-ctx.Done():
				return
			}

			// tail -F: reopen if the file was recreated or truncated.
			if nf, rotated := reopenIfRotated(f, path, offset); rotated {
				f = nf
				offset = 0
				partial = nil
			}
		case readErr != nil:
			return
		}
	}
}

// reopenIfRotated returns a fresh handle (and true) when the file at path
// was recreated (new inode) or truncated below offset; otherwise the
// original handle and false.
func reopenIfRotated(f *os.File, path string, offset int64) (*os.File, bool) {
	pathInfo, err := os.Stat(path)
	if err != nil {
		// Gone for the moment — keep waiting on the current handle.
		return f, false
	}

	if curInfo, err := f.Stat(); err == nil && os.SameFile(pathInfo, curInfo) {
		if pathInfo.Size() >= offset {
			return f, false // same file, not truncated
		}

		// Truncated in place — rewind to the start.
		if _, err := f.Seek(0, io.SeekStart); err == nil {
			return f, true
		}

		return f, false
	}

	// Recreated with a new inode — reopen.
	nf, err := os.Open(path)
	if err != nil {
		return f, false
	}

	defer func() { f.Close() }() //nolint:errcheck

	return nf, true
}
