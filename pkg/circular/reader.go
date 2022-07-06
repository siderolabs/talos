// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package circular

import (
	"io"
	"sync/atomic"
)

// Reader implements seekable reader with local position in the Buffer which
// reads from the fixed part of the buffer.
//
// Reader is not safe to be used with concurrent Read/Seek operations.
type Reader struct {
	buf *Buffer

	startOff, endOff int64
	off              int64

	closed uint32
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	if atomic.LoadUint32(&r.closed) > 0 {
		err = ErrClosed

		return
	}

	r.buf.mu.Lock()
	defer r.buf.mu.Unlock()

	if r.off < r.buf.off-int64(r.buf.opt.MaxCapacity) {
		// reader is falling too much behind
		err = ErrOutOfSync

		return
	}

	if r.off == r.endOff {
		err = io.EOF

		return
	}

	if len(p) == 0 {
		return
	}

	n = int(r.endOff - r.off)
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
func (r *Reader) Close() error {
	atomic.StoreUint32(&r.closed, 1)

	return nil
}

// Seek implements io.Seeker.
func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	newOff := r.off

	switch whence {
	case io.SeekCurrent:
		newOff += offset
	case io.SeekEnd:
		newOff = r.endOff + offset
	case io.SeekStart:
		newOff = r.startOff + offset
	}

	if newOff < r.startOff {
		return r.off - r.startOff, ErrSeekBeforeStart
	}

	if newOff > r.endOff {
		newOff = r.endOff
	}

	r.off = newOff

	return r.off - r.startOff, nil
}
