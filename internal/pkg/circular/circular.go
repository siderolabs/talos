// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package circular provides a buffer with circular semantics.
package circular

import (
	"fmt"
	"sync"
)

// Buffer implements circular buffer which supports single writer and multiple
// readers each with its own offset.
type Buffer struct {
	opt Options

	// synchronizing access to data, off
	mu   sync.Mutex
	cond *sync.Cond

	// data slice, might grow up to MaxCapacity, then used
	// as circular buffer
	data []byte

	// write offset, always goes up, actual offset in data slice
	// is (off % cap(data))
	off int64
}

// NewBuffer creates new Buffer with specified options.
func NewBuffer(opts ...OptionFunc) (*Buffer, error) {
	buf := &Buffer{
		opt: defaultOptions(),
	}

	for _, o := range opts {
		if err := o(&buf.opt); err != nil {
			return nil, err
		}
	}

	if buf.opt.InitialCapacity > buf.opt.MaxCapacity {
		return nil, fmt.Errorf("initial capacity (%d) should be less or equal to max capacity (%d)", buf.opt.InitialCapacity, buf.opt.MaxCapacity)
	}

	if buf.opt.SafetyGap >= buf.opt.MaxCapacity {
		return nil, fmt.Errorf("safety gap (%d) should be less than max capacity (%d)", buf.opt.SafetyGap, buf.opt.MaxCapacity)
	}

	buf.data = make([]byte, buf.opt.InitialCapacity)
	buf.cond = sync.NewCond(&buf.mu)

	return buf, nil
}

// Write implements io.Writer interface.
func (buf *Buffer) Write(p []byte) (n int, err error) {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	l := len(p)

	if buf.off < int64(buf.opt.MaxCapacity) {
		if buf.off+int64(l) > int64(cap(buf.data)) && cap(buf.data) < buf.opt.MaxCapacity {
			// grow buffer to ensure write fits, but limit with max capacity
			size := cap(buf.data) * 2
			for size < int(buf.off)+l {
				size *= 2
			}

			if size > buf.opt.MaxCapacity {
				size = buf.opt.MaxCapacity
			}

			data := make([]byte, size)
			copy(data, buf.data)
			buf.data = data
		}
	}

	for n < l {
		i := int(buf.off % int64(buf.opt.MaxCapacity))

		nn := buf.opt.MaxCapacity - i
		if nn > len(p) {
			nn = len(p)
		}

		copy(buf.data[i:], p[:nn])

		buf.off += int64(nn)
		n += nn
		p = p[nn:]
	}

	if n > 0 {
		buf.cond.Broadcast()
	}

	return n, err
}

// Capacity returns number of bytes allocated for the buffer.
func (buf *Buffer) Capacity() int {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	return cap(buf.data)
}

// Offset returns current write offset (number of bytes written).
func (buf *Buffer) Offset() int64 {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	return buf.off
}

// GetStreamingReader returns StreamingReader object which implements io.ReadCloser, io.Seeker.
//
// StreamingReader starts at the most distant position in the past available.
func (buf *Buffer) GetStreamingReader() *StreamingReader {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	off := buf.off - int64(buf.opt.MaxCapacity-buf.opt.SafetyGap)
	if off < 0 {
		off = 0
	}

	return &StreamingReader{
		buf:        buf,
		initialOff: off,
		off:        off,
	}
}

// GetReader returns Reader object which implements io.ReadCloser, io.Seeker.
//
// Reader starts at the most distant position in the past available and goes
// to the current write position.
func (buf *Buffer) GetReader() *Reader {
	buf.mu.Lock()
	defer buf.mu.Unlock()

	off := buf.off - int64(buf.opt.MaxCapacity-buf.opt.SafetyGap)
	if off < 0 {
		off = 0
	}

	return &Reader{
		buf:      buf,
		startOff: off,
		endOff:   buf.off,
		off:      off,
	}
}
