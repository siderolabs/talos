/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package archiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-multierror"
)

// FileItem is unit of work for archive
type FileItem struct {
	FullPath string
	RelPath  string
	FileInfo os.FileInfo
	Link     string
}

// Walker provides a channel of file info/paths for archival
//
//nolint: gocyclo
func Walker(ctx context.Context, rootPath string) (<-chan FileItem, <-chan error, error) {
	_, err := os.Stat(rootPath)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan FileItem)
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)

		multiErr := &multierror.Error{}

		defer func() {
			errCh <- multiErr.ErrorOrNil()
		}()

		err := filepath.Walk(rootPath, func(path string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				multiErr = multierror.Append(multiErr, walkErr)
				return nil
			}

			var (
				relPath string
				err     error
			)

			if path == rootPath {
				if fileInfo.IsDir() {
					// skip containing directory
					return nil
				}

				// only one file
				relPath = filepath.Base(path)
			} else {
				relPath, err = filepath.Rel(rootPath, path)
				if err != nil {
					return err
				}
			}

			item := FileItem{
				FullPath: path,
				RelPath:  relPath,
				FileInfo: fileInfo,
			}

			if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
				item.Link, err = os.Readlink(path)
				if err != nil {
					multiErr = multierror.Append(multiErr, fmt.Errorf("error reading symlink %q: %s", path, err))
					return nil
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- item:
			}

			return nil
		})

		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}

	}()

	return ch, errCh, nil
}
