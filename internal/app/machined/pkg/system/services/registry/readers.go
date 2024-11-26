// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/errdefs"
)

var (
	errInvalidSize            = errors.New("readerat: invalid size")
	errSeekToInvalidWhence    = errors.New("readerat: seek to invalid whence")
	errSeekToNegativePosition = errors.New("readerat: seek to negative position")
)

// readSeeker is an io.ReadSeeker implementation based on an io.ReaderAt (and
// an int64 size).
//
// For example, an os.File is both an io.ReaderAt and an io.ReadSeeker, but its
// io.ReadSeeker methods are not safe to use concurrently. In comparison,
// multiple readerat.readSeeker values (using the same os.File as their
// io.ReaderAt) are safe to use concurrently. Each can Read and Seek
// independently.
//
// A single readerat.readSeeker is not safe to use concurrently.
//
// Do not modify its exported fields after calling any of its methods.
type readSeeker struct {
	ReaderAt io.ReaderAt
	Size     int64
	offset   int64
}

// Read implements io.Reader.
func (r *readSeeker) Read(p []byte) (int, error) {
	if r.Size < 0 {
		return 0, errInvalidSize
	} else if r.Size <= r.offset {
		return 0, io.EOF
	}

	if length := r.Size - r.offset; int64(len(p)) > length {
		p = p[:length]
	}

	if len(p) == 0 {
		return 0, nil
	}

	actual, err := r.ReaderAt.ReadAt(p, r.offset)
	r.offset += int64(actual)

	if err == nil && r.offset == r.Size {
		err = io.EOF
	}

	return actual, err
}

// Seek implements io.Seeker.
func (r *readSeeker) Seek(offset int64, whence int) (int64, error) {
	if r.Size < 0 {
		return 0, errInvalidSize
	}

	switch whence {
	case io.SeekStart:
		// No-op.
	case io.SeekCurrent:
		offset += r.offset
	case io.SeekEnd:
		offset += r.Size
	default:
		return 0, errSeekToInvalidWhence
	}

	if offset < 0 {
		return 0, errSeekToNegativePosition
	}

	r.offset = offset

	return r.offset, nil
}

// openReaderAt creates ReaderAt from a file.
func openReaderAt(p string, statFS fs.StatFS) (content.ReaderAt, error) {
	fi, err := statFS.Stat(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		return nil, fmt.Errorf("blob not found: %w", errdefs.ErrNotFound)
	}

	fp, err := statFS.Open(p)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		return nil, fmt.Errorf("blob not found: %w", errdefs.ErrNotFound)
	}

	f, ok := fp.(fsFileReaderAt)
	if !ok {
		return nil, fmt.Errorf("not a fsFileReaderAt: %T, details: %v", fp, fp)
	}

	return sizeReaderAt{size: fi.Size(), fp: f}, nil
}

// readerat implements io.ReaderAt in a completely stateless manner by opening
// the referenced file for each call to ReadAt.
type sizeReaderAt struct {
	size int64
	fp   fsFileReaderAt
}

func (ra sizeReaderAt) ReadAt(p []byte, offset int64) (int, error) { return ra.fp.ReadAt(p, offset) }
func (ra sizeReaderAt) Size() int64                                { return ra.size }
func (ra sizeReaderAt) Close() error                               { return ra.fp.Close() }
func (ra sizeReaderAt) Reader() io.Reader                          { return io.LimitReader(ra.fp, ra.size) }

type fsFileReaderAt interface {
	io.ReaderAt
	fs.File
}
