// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

//nolint:gocyclo
func (p *provisioner) createPFlashImages(state *vm.State, nodeName string, pflashSpec []PFlash, secureBootEnabled bool) ([]string, error) {
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

	if secureBootEnabled {
		flashVarsPath := state.GetRelativePath(fmt.Sprintf("%s-flash_vars.fd", nodeName))

		cmd := exec.Command("ovmfctl", []string{
			"--no-microsoft",
			"--secure-boot",
			"--set-pk",
			// OEM value from here: https://bugzilla.tianocore.org/show_bug.cgi?id=1747#c2
			"4e32566d-8e9e-4f52-81d3-5bb9715f9727",
			"hack/certs/uki-signing.crt",
			"--add-kek",
			"4e32566d-8e9e-4f52-81d3-5bb9715f9727",
			"hack/certs/uki-signing.crt",
			"--add-db",
			"4e32566d-8e9e-4f52-81d3-5bb9715f9727",
			"hack/certs/uki-signing.crt",
			"--input",
			"/usr/share/OVMF/OVMF_VARS_4M.fd",
			"--output",
			flashVarsPath,
		}...)

		if err := cmd.Run(); err != nil {
			return nil, err
		}

		images = append(images, flashVarsPath)
	}

	return images, nil
}
