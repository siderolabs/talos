// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lastlog provides utilities for capturing last log line(s) with a writer.
package lastlog

import (
	"bytes"
	"sync"
)

// limit is the maximum number of bytes to capture.
const limit = 512

// Writer is a writer that captures the last log line(s).
type Writer struct {
	mu  sync.Mutex
	buf []byte
}

// Writer implements io.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf == nil {
		w.buf = make([]byte, 0, limit)
	}

	if len(p) >= limit {
		cut := p

		for len(cut) > limit {
			nlIndex := bytes.IndexByte(cut, '\n')
			if nlIndex == -1 {
				// no newline found, append the last part of the cut
				if len(cut) > limit {
					cut = cut[len(cut)-limit:]
				}

				break
			}

			cut = cut[nlIndex+1:]
		}

		w.buf = append(w.buf[:0], cut...)

		return len(p), nil
	}

	bufPos := 0

	for len(w.buf)-bufPos+len(p) > limit {
		nlIndex := bytes.IndexByte(w.buf[bufPos:], '\n')
		if nlIndex == -1 {
			// no newline found, drop the whole buffer
			bufPos = len(w.buf)
		}

		// we are over the limit, drop the beginning of the buffer
		bufPos += nlIndex + 1
	}

	if bufPos > 0 {
		// drop the beginning of the buffer
		copy(w.buf, w.buf[bufPos:])
		w.buf = w.buf[:len(w.buf)-bufPos]
	}

	w.buf = append(w.buf, p...)

	return len(p), nil
}

// GetLastLog returns the last log line(s) captured by the writer.
func (w *Writer) GetLastLog() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return string(bytes.TrimRight(w.buf, "\n"))
}
