// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package archiver

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/talos-systems/talos/pkg/safepath"
)

// Untar extracts .tar archive from r into filesystem under rootPath.
//
//nolint:gocyclo
func Untar(ctx context.Context, r io.Reader, rootPath string) error {
	tr := tar.NewReader(r)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

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

		path := filepath.Join(rootPath, hdrPath)

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
