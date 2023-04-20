// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package archiver

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"

	multierror "github.com/hashicorp/go-multierror"
)

// Tar creates .tar archive and writes it to output for every item in paths channel.
func Tar(ctx context.Context, paths <-chan FileItem, output io.Writer) error {
	tw := tar.NewWriter(output)
	//nolint:errcheck
	defer tw.Close()

	var multiErr *multierror.Error

	buf := make([]byte, 4096)

	for fi := range paths {
		if fi.Error != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("skipping %q: %s", fi.FullPath, fi.Error))

			continue
		}

		err := processFile(ctx, tw, fi, buf)
		if err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("skipping %q: %s", fi.FullPath, err))
		}
	}

	if err := tw.Close(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	return multiErr.ErrorOrNil()
}

//nolint:gocyclo
func processFile(ctx context.Context, tw *tar.Writer, fi FileItem, buf []byte) error {
	header, err := tar.FileInfoHeader(fi.FileInfo, fi.Link)
	if err != nil {
		// not supported by tar
		return err
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

	var r io.Reader

	if !skipData {
		var fp *os.File

		fp, err = os.Open(fi.FullPath)
		if err != nil {
			return err
		}

		defer fp.Close() //nolint:errcheck

		r = fp
	}

	if !skipData && header.Size == 0 {
		// Linux reports /proc files as zero length, but they might have data,
		// so we try to read limited amount of data from it to determine the size
		var n int

		n, err = r.Read(buf)

		switch {
		case err == io.EOF:
			// file is empty for real
			skipData = true
		case err != nil:
			// error reading from the file
			if errors.Is(err, syscall.EINVAL) {
				// some files are not supported by os.Open, e.g. /proc/sys/net/ipv4/conf/all/accept_local
				skipData = true
			} else {
				return err
			}
		case n < len(buf):
			header.Size = int64(n)
			r = bytes.NewReader(append([]byte(nil), buf[:n]...))
		default:
			// none matched so the file is bigger than we expected, ignore it and copy as zero size
			skipData = true
		}
	}

	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	if skipData {
		return nil
	}

	return archiveFile(ctx, tw, fi, r, buf)
}

func archiveFile(ctx context.Context, tw io.Writer, fi FileItem, r io.Reader, buf []byte) error {
	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
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
}
