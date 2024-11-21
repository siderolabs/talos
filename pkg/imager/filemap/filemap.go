// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package filemap provides a way to create reproducible layers from a file system.
package filemap

import (
	"archive/tar"
	"cmp"
	"fmt"
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

		rel, err := filepath.Rel(sourceBasePath, path)
		if err != nil {
			return err
		}

		if d.IsDir() && rel == "." {
			return nil
		}

		statInfo, err := d.Info()
		if err != nil {
			return err
		}

		filemap = append(filemap, File{
			ImagePath:  filepath.Join(imageBasePath, rel),
			SourcePath: path,
			ImageMode:  int64(statInfo.Mode().Perm()),
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
						return fmt.Errorf("error opening %q: %w", entry.SourcePath, err)
					}

					defer in.Close() //nolint:errcheck

					st, err := in.Stat()
					if err != nil {
						return fmt.Errorf("error stating file %s: %w", entry.SourcePath, err)
					}

					if st.IsDir() {
						if err := handleDir(w, entry.ImagePath, entry.ImageMode); err != nil {
							return fmt.Errorf("error handling directory %s: %w", entry.SourcePath, err)
						}

						return in.Close()
					}

					if err := handleFile(w, in, entry.ImagePath, entry.ImageMode, st.Size()); err != nil {
						return fmt.Errorf("error handling file %s: %w", entry.SourcePath, err)
					}

					return in.Close()
				}(entry); err != nil {
					return fmt.Errorf("error processing %s: %w", entry.SourcePath, err)
				}
			}

			return w.Close()
		}())
	}()

	return pr
}

func handleFile(w *tar.Writer, r io.Reader, path string, mode, size int64) error {
	header := &tar.Header{
		Name: path,
		Mode: mode,
		Size: size,
	}

	if err := w.WriteHeader(header); err != nil {
		return fmt.Errorf("error writing tar header for %s: %w", path, err)
	}

	if _, err := io.Copy(w, r); err != nil {
		return fmt.Errorf("error writing tar data for %s: %w", path, err)
	}

	return nil
}

func handleDir(w *tar.Writer, path string, mode int64) error {
	header := &tar.Header{
		Name:     path,
		Mode:     mode,
		Typeflag: tar.TypeDir,
	}

	if err := w.WriteHeader(header); err != nil {
		return fmt.Errorf("error writing tar header for %s: %w", path, err)
	}

	return nil
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
