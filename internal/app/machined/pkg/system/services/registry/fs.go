// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"errors"
	"io/fs"
	"iter"

	"github.com/hashicorp/go-multierror"
)

// MultiPathFS is a FS that can be used to combine multiple FSs into one.
type MultiPathFS struct {
	fsIt iter.Seq[fs.StatFS]
}

// NewMultiPathFS creates a new MultiPathFS. It takes an iterator of FSs which can be used multiple times asynchrously.
func NewMultiPathFS(it iter.Seq[fs.StatFS]) *MultiPathFS { return &MultiPathFS{fsIt: it} }

// Open opens the named file.
func (m *MultiPathFS) Open(name string) (fs.File, error) {
	var multiErr *multierror.Error

	for f := range m.fsIt {
		r, err := f.Open(name)
		if err == nil {
			return r, nil
		}

		multiErr = multierror.Append(multiErr, err)
	}

	if multiErr == nil {
		return nil, errors.New("roots are empty")
	}

	return nil, multiErr.ErrorOrNil()
}

// Stat returns a [fs.FileInfo] describing the named file.
func (m *MultiPathFS) Stat(name string) (fs.FileInfo, error) {
	var multiErr *multierror.Error

	for f := range m.fsIt {
		r, err := f.Stat(name)
		if err == nil {
			return r, nil
		}

		multiErr = multierror.Append(multiErr, err)
	}

	if multiErr == nil {
		return nil, errors.New("roots are empty")
	}

	return nil, multiErr.ErrorOrNil()
}
