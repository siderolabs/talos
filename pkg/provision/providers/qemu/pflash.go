// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"fmt"
	"io"
	"os"

	"github.com/talos-systems/talos/pkg/provision/providers/vm"
)

//nolint:gocyclo
func (p *provisioner) createPFlashImages(state *vm.State, nodeName string, pflashSpec []PFlash) ([]string, error) {
	var images []string

	for i, pflash := range pflashSpec {
		if err := func(i int, pflash PFlash) error {
			path := state.GetRelativePath(fmt.Sprintf("%s-flash%d.img", nodeName, i))

			f, err := os.Create(path)
			if err != nil {
				return err
			}

			defer f.Close() //nolint:errcheck

			if err = f.Truncate(pflash.Size); err != nil {
				return err
			}

			if pflash.SourcePaths != nil {
				for _, sourcePath := range pflash.SourcePaths {
					var src *os.File

					src, err = os.Open(sourcePath)
					if err != nil {
						if os.IsNotExist(err) {
							continue
						}

						return err
					}

					defer src.Close() //nolint:errcheck

					if _, err = io.Copy(f, src); err != nil {
						return err
					}

					break
				}

				if err != nil {
					return err
				}
			}

			images = append(images, path)

			return nil
		}(i, pflash); err != nil {
			return nil, err
		}
	}

	return images, nil
}
