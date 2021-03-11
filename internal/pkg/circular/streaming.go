// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package circular

import (
	"io"
	"sync/atomic"
)

// StreamingReader implements seekable reader with local position in the Buffer.
//
// StreamingReader is not safe to be used with concurrent Read/Seek operations.
//
// StreamingReader blocks for new data once it exhausts contents of the buffer.
type StreamingReader struct {
	buf *Buffer

	initialOff int64
	off        int64

	closed uint32
}

// Read implements io.Reader.
func (r *StreamingReader) Read(p []byte) (n int, err error) {
	if atomic.LoadUint32(&r.closed) > 0 {
		err = ErrClosed

		return
	}

	if len(p) == 0 {
		return
	}

	r.buf.mu.Lock()
	defer r.buf.mu.Unlock()

	if r.off < r.buf.off-int64(r.buf.opt.MaxCapacity) {
		// reader is falling too much behind, so need to rewind to the first available position
		r.off = r.buf.off - int64(r.buf.opt.MaxCapacity)
	}

	for r.off == r.buf.off {
		r.buf.cond.Wait()

		if atomic.LoadUint32(&r.closed) > 0 {
			err = ErrClosed

			return
		}
	}

	n = int(r.buf.off - r.off)
	if n > len(p) {
		n = len(p)
	}

	i := int(r.off % int64(r.buf.opt.MaxCapacity))

	if l := r.buf.opt.MaxCapacity - i; l < n {
		copy(p, r.buf.data[i:])
		copy(p[l:], r.buf.data[:n-l])
	} else {
		copy(p, r.buf.data[i:i+n])
	}

	r.off += int64(n)

	return n, err
}

// Close implements io.Closer.
func (r *StreamingReader) Close() error {
	if atomic.CompareAndSwapUint32(&r.closed, 0, 1) {
		// wake up readers
		r.buf.cond.Broadcast()
	}

	return nil
}

// Seek implements io.Seeker.
func (r *StreamingReader) Seek(offset int64, whence int) (int64, error) {
	newOff := r.off

	r.buf.mu.Lock()
	writeOff := r.buf.off
	r.buf.mu.Unlock()

	switch whence {
	case io.SeekCurrent:
		newOff += offset
	case io.SeekEnd:
		newOff = writeOff + offset
	case io.SeekStart:
		newOff = r.initialOff + offset
	}

	if newOff < r.initialOff {
		return r.off - r.initialOff, ErrSeekBeforeStart
	}

	if newOff > writeOff {
		newOff = writeOff
	}

	if newOff < writeOff-int64(r.buf.opt.MaxCapacity-r.buf.opt.SafetyGap) {
		newOff = writeOff - int64(r.buf.opt.MaxCapacity-r.buf.opt.SafetyGap)
	}

	r.off = newOff

	return r.off - r.initialOff, nil
}
