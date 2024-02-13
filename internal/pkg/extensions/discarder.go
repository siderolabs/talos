// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"errors"
	"io"
)

// discarder is used to implement ReadAt from a Reader
// by reading, and discarding, data until the offset
// is reached. It can only go forward. It is designed
// for pipe-like files.
type discarder struct {
	r   io.Reader
	pos int64
}

// ReadAt implements ReadAt for a discarder.
// It is an error for the offset to be negative.
func (r *discarder) ReadAt(p []byte, off int64) (int, error) {
	if off-r.pos < 0 {
		return 0, errors.New("negative seek on discarder not allowed")
	}

	if off != r.pos {
		i, err := io.Copy(io.Discard, io.LimitReader(r.r, off-r.pos))
		if err != nil || i != off-r.pos {
			return 0, err
		}

		r.pos += i
	}

	n, err := io.ReadFull(r.r, p)
	if err != nil {
		return n, err
	}

	r.pos += int64(n)

	return n, err
}

var _ io.ReaderAt = &discarder{}
