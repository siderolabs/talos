// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/siderolabs/talos/pkg/provision"
)

func (p *provisioner) createPFlashImages(state *provision.State, nodeName string, pflashSpec []PFlash) ([]string, error) {
	images := make([]string, 0, len(pflashSpec))

	for i, pflash := range pflashSpec {
		path := state.GetRelativePath(fmt.Sprintf("%s-flash%d.img", nodeName, i))

		if err := writePFlashImage(path, pflash); err != nil {
			return nil, fmt.Errorf("failed to write pflash image %s: %w", path, err)
		}

		images = append(images, path)
	}

	return images, nil
}

// writePFlashImage materializes a single pflash image at path from the first
// readable source in pflash.SourcePaths.
func writePFlashImage(path string, pflash PFlash) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	if err = f.Truncate(pflash.Size); err != nil {
		return err
	}

	if len(pflash.SourcePaths) == 0 {
		return nil
	}

	for _, sourcePath := range pflash.SourcePaths {
		src, err := os.Open(sourcePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return err
		}

		var r io.Reader = src
		if pflash.Size > 0 {
			r = io.LimitReader(src, pflash.Size)
		}

		_, copyErr := io.Copy(f, r)

		src.Close() //nolint:errcheck

		if copyErr != nil {
			return copyErr
		}

		return nil
	}

	return fmt.Errorf("no readable pflash source found in %v", pflash.SourcePaths)
}
