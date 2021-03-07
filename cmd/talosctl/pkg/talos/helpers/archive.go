// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/talos-systems/talos/pkg/safepath"
)

// ExtractFileFromTarGz reads a single file data from an archive.
func ExtractFileFromTarGz(filename string, r io.ReadCloser) ([]byte, error) {
	defer r.Close() //nolint:errcheck

	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("error initializing gzip: %w", err)
	}

	tr := tar.NewReader(zr)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		hdrPath := safepath.CleanPath(hdr.Name)
		if hdrPath == "" {
			return nil, fmt.Errorf("empty tar header path")
		}

		if hdrPath == filename {
			if hdr.Typeflag == tar.TypeDir || hdr.Typeflag == tar.TypeSymlink {
				return nil, fmt.Errorf("%s is not a file", filename)
			}

			return ioutil.ReadAll(tr)
		}
	}

	return nil, fmt.Errorf("couldn't find file %s in the archive", filename)
}

// ExtractTarGz extracts .tar.gz archive from r into filesystem under localPath.
//
//nolint:gocyclo
func ExtractTarGz(localPath string, r io.ReadCloser) error {
	defer r.Close() //nolint:errcheck

	zr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("error initializing gzip: %w", err)
	}

	tr := tar.NewReader(zr)

	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return fmt.Errorf("error reading tar header: %s", err)
		}

		hdrPath := safepath.CleanPath(hdr.Name)
		if hdrPath == "" {
			return fmt.Errorf("empty tar header path")
		}

		path := filepath.Join(localPath, hdrPath)
		// TODO: do we need to clean up any '..' references?

		switch hdr.Typeflag {
		case tar.TypeDir:
			mode := hdr.FileInfo().Mode()
			mode |= 0o700 // make rwx for the owner

			if err = os.Mkdir(path, mode); err != nil {
				return fmt.Errorf("error creating directory %q mode %s: %w", path, mode, err)
			}

			if err = os.Chmod(path, mode); err != nil {
				return fmt.Errorf("error updating mode %s for %q: %w", mode, path, err)
			}

		case tar.TypeSymlink:
			if err = os.Symlink(hdr.Linkname, path); err != nil {
				return fmt.Errorf("error creating symlink %q -> %q: %w", path, hdr.Linkname, err)
			}

		default:
			mode := hdr.FileInfo().Mode()

			fp, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
			if err != nil {
				return fmt.Errorf("error creating file %q mode %s: %w", path, mode, err)
			}

			_, err = io.Copy(fp, tr)
			if err != nil {
				return fmt.Errorf("error copying data to %q: %w", path, err)
			}

			if err = fp.Close(); err != nil {
				return fmt.Errorf("error closing %q: %w", path, err)
			}

			if err = os.Chmod(path, mode); err != nil {
				return fmt.Errorf("error updating mode %s for %q: %w", mode, path, err)
			}
		}
	}

	return nil
}
