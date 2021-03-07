// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package archiver

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	multierror "github.com/hashicorp/go-multierror"
)

// Tar creates .tar archive and writes it to output for every item in paths channel
//
//nolint:gocyclo
func Tar(ctx context.Context, paths <-chan FileItem, output io.Writer) error {
	tw := tar.NewWriter(output)
	//nolint:errcheck
	defer tw.Close()

	var multiErr *multierror.Error

	for fi := range paths {
		if fi.Error != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("skipping %q: %s", fi.FullPath, fi.Error))

			continue
		}

		header, err := tar.FileInfoHeader(fi.FileInfo, fi.Link)
		if err != nil {
			// not supported by tar
			multiErr = multierror.Append(multiErr, fmt.Errorf("skipping %q: %s", fi.FullPath, err))

			continue
		}

		header.Name = fi.RelPath
		if fi.FileInfo.IsDir() {
			header.Name += string(os.PathSeparator)
		}

		skipData := false

		switch header.Typeflag {
		case tar.TypeLink, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeDir, tar.TypeFifo:
			// no data for these types, move on
			skipData = true
		}

		if header.Size == 0 {
			// skip files with zero length
			//
			// this might skip contents for special files in /proc, but
			// anyways we can't archive them properly if we don't know size beforehand
			skipData = true
		}

		var fp *os.File
		if !skipData {
			fp, err = os.Open(fi.FullPath)
			if err != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("skipping %q: %s", fi.FullPath, err))

				continue
			}
		}

		err = tw.WriteHeader(header)
		if err != nil {
			//nolint:errcheck
			fp.Close()

			multiErr = multierror.Append(multiErr, err)

			return multiErr
		}

		if !skipData {
			err = archiveFile(ctx, tw, fi, fp)
			if err != nil {
				multiErr = multierror.Append(multiErr, err)

				return multiErr
			}
		}
	}

	if err := tw.Close(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	return multiErr.ErrorOrNil()
}

func archiveFile(ctx context.Context, tw io.Writer, fi FileItem, fp *os.File) error {
	//nolint:errcheck
	defer fp.Close()

	buf := make([]byte, 4096)

	for {
		n, err := fp.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err = tw.Write(buf[:n])
		if err != nil {
			if err == tar.ErrWriteTooLong {
				log.Printf("ignoring long write for %q", fi.FullPath)

				return nil
			}

			return err
		}
	}

	return fp.Close()
}
