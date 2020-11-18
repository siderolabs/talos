// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
)

// ExtractFileFromTarGz reads a single file data from an archive.
func ExtractFileFromTarGz(filename string, r io.Reader) (io.Reader, error) {
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

		if hdr.Name == filename {
			if hdr.Typeflag == tar.TypeDir || hdr.Typeflag == tar.TypeSymlink {
				return nil, fmt.Errorf("%s is not a file", filename)
			}

			return tr, nil
		}
	}

	return nil, fmt.Errorf("couldn't find file %s in the archive", filename)
}
