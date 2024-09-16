// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroups

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
)

// TreeFromTarGz builds a crgroup tree from the tar.gz reader.
//
// It is assumed to work with output of `talosctl cp /sys/fs/cgroup -`.
func TreeFromTarGz(r io.Reader) (*Tree, error) {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	defer gzReader.Close() //nolint:errcheck

	tarReader := tar.NewReader(gzReader)

	tree := &Tree{
		Root: &Node{},
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		directory, filename := filepath.Split(header.Name)

		node := tree.Find(directory)

		if err = node.Parse(filename, tarReader); err != nil {
			return nil, fmt.Errorf("failed to parse %q: %w", header.Name, err)
		}
	}

	return tree, nil
}
