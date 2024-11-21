// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"io/fs"
	"iter"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
)

// MultiPathFS is a [fs.FS] that reads from multiple paths sequentially until it finds the file.
type MultiPathFS struct {
	fsIt iter.Seq[string]
}

// NewMultiPathFS creates a new MultiPathFS. It takes an iterator of FSs which can be used multiple times asynchrously.
func NewMultiPathFS(it iter.Seq[string]) *MultiPathFS { return &MultiPathFS{fsIt: it} }

// Open opens the named file.
func (m *MultiPathFS) Open(name string) (fs.File, error) {
	var multiErr *multierror.Error

	for root := range m.fsIt {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}

		r, err := os.Open(filepath.Join(abs, name))
		if err == nil {
			return r, nil
		}

		multiErr = multierror.Append(multiErr, err)
	}

	if multiErr == nil {
		return nil, os.ErrNotExist
	}

	return nil, multiErr.ErrorOrNil()
}

// Stat returns a [fs.FileInfo] describing the named file.
func (m *MultiPathFS) Stat(name string) (fs.FileInfo, error) {
	var multiErr *multierror.Error

	for root := range m.fsIt {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}

		r, err := os.Stat(filepath.Join(abs, name))
		if err == nil {
			return r, nil
		}

		multiErr = multierror.Append(multiErr, err)
	}

	if multiErr == nil {
		return nil, os.ErrNotExist
	}

	return nil, multiErr.ErrorOrNil()
}
