// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"fmt"
	"io"
	"os"

	"github.com/talos-systems/talos/internal/pkg/provision/providers/vm"
)

func (p *provisioner) createPFlashImages(state *vm.State, pflashSpec []PFlash) error {
	for i, pflash := range pflashSpec {
		if err := func(i int, pflash PFlash) error {
			path := state.GetRelativePath(fmt.Sprintf("flash%d.img", i))

			f, err := os.Create(path)
			if err != nil {
				return nil
			}

			defer f.Close() //nolint: errcheck

			if err := f.Truncate(pflash.Size); err != nil {
				return err
			}

			if pflash.SourcePath != "" {
				src, err := os.Open(pflash.SourcePath)
				if err != nil {
					return nil
				}

				defer src.Close() //nolint: errcheck

				if _, err := io.Copy(f, src); err != nil {
					return err
				}
			}

			state.PFlashImages = append(state.PFlashImages, path)

			return nil
		}(i, pflash); err != nil {
			return err
		}
	}

	return nil
}
