// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package filemap provides a way to create reproducible layers from a file system.
package filemap

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"sort"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// File is a path -> file content map representing a file system.
type File struct {
	ImagePath  string
	SourcePath string
}

// Layer creates a layer from a single file map.
//
// These layers are reproducible and consistent.
//
// A filemap is a path -> file content map representing a file system.
func Layer(filemap []File) (v1.Layer, error) {
	b := &bytes.Buffer{}
	w := tar.NewWriter(b)

	sort.Slice(filemap, func(i, j int) bool {
		return filemap[i].ImagePath < filemap[j].ImagePath
	})

	for _, entry := range filemap {
		if err := func(entry File) error {
			in, err := os.Open(entry.SourcePath)
			if err != nil {
				return err
			}

			defer in.Close() //nolint:errcheck

			st, err := in.Stat()
			if err != nil {
				return err
			}

			if err = w.WriteHeader(&tar.Header{
				Name: entry.ImagePath,
				Size: st.Size(),
			}); err != nil {
				return err
			}

			_, err = io.Copy(w, in)
			if err != nil {
				return err
			}

			return in.Close()
		}(entry); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	// Return a new copy of the buffer each time it's opened.
	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(b.Bytes())), nil
	})
}
