// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package filemap provides a way to create reproducible layers from a file system.
package filemap

import (
	"archive/tar"
	"cmp"
	"io"
	"os"
	"path/filepath"
	"slices"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// File is a path -> file content map representing a file system.
type File struct {
	ImagePath  string
	SourcePath string
	ImageMode  int64
}

// Walk the filesystem generating a filemap.
func Walk(sourceBasePath, imageBasePath string) ([]File, error) {
	var filemap []File

	err := filepath.WalkDir(sourceBasePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(sourceBasePath, path)
		if err != nil {
			return err
		}

		filemap = append(filemap, File{
			ImagePath:  filepath.Join(imageBasePath, rel),
			SourcePath: path,
		})

		return nil
	})

	return filemap, err
}

func build(filemap []File) io.ReadCloser {
	pr, pw := io.Pipe()

	go func() {
		pw.CloseWithError(func() error {
			w := tar.NewWriter(pw)

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
						Mode: entry.ImageMode,
					}); err != nil {
						return err
					}

					_, err = io.Copy(w, in)
					if err != nil {
						return err
					}

					return in.Close()
				}(entry); err != nil {
					return err
				}
			}

			return w.Close()
		}())
	}()

	return pr
}

// Layer creates a layer from a single file map.
//
// These layers are reproducible and consistent.
//
// A filemap is a path -> file content map representing a file system.
func Layer(filemap []File) (v1.Layer, error) {
	slices.SortFunc(filemap, func(a, b File) int { return cmp.Compare(a.ImagePath, b.ImagePath) })

	// Return a new copy of the buffer each time it's opened.
	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return build(filemap), nil
	})
}
