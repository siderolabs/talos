// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package archiver provides a service to archive part of the filesystem into tar archive.
package archiver

import (
	"compress/gzip"
	"context"
	"io"
)

// TarGz produces .tar.gz archive of filesystem starting at rootPath.
func TarGz(ctx context.Context, rootPath string, output io.Writer, walkerOptions ...WalkerOption) error {
	paths, err := Walker(ctx, rootPath, append(walkerOptions, WithSkipRoot())...)
	if err != nil {
		return err
	}

	zw := gzip.NewWriter(output)
	//nolint:errcheck
	defer zw.Close()

	err = Tar(ctx, paths, zw)
	if err != nil {
		return err
	}

	return zw.Close()
}

// UntarGz extracts .tar.gz archive to the rootPath.
func UntarGz(ctx context.Context, input io.Reader, rootPath string) error {
	zr, err := gzip.NewReader(input)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer zr.Close()

	err = Untar(ctx, zr, rootPath)
	if err != nil {
		return err
	}

	return zr.Close()
}
