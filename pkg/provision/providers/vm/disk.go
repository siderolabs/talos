// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"os"

	"github.com/talos-systems/talos/pkg/provision"
)

// UserDiskName returns disk device path.
func (p *Provisioner) UserDiskName(index int) string {
	res := "/dev/vd"

	var convert func(i int) string

	convert = func(i int) string {
		remainder := i % 26
		divider := i / 26

		prefix := ""

		if divider != 0 {
			prefix = convert(divider - 1)
		}

		return fmt.Sprintf("%s%s", prefix, string(rune('a'+remainder)))
	}

	return res + convert(index)
}

// CreateDisks creates empty disk files for each disk.
func (p *Provisioner) CreateDisks(state *State, nodeReq provision.NodeRequest) (diskPaths []string, err error) {
	diskPaths = make([]string, len(nodeReq.Disks))

	for i, disk := range nodeReq.Disks {
		diskPath := state.GetRelativePath(fmt.Sprintf("%s-%d.disk", nodeReq.Name, i))

		var diskF *os.File

		diskF, err = os.Create(diskPath)
		if err != nil {
			return
		}

		defer diskF.Close() //nolint:errcheck

		err = diskF.Truncate(int64(disk.Size))
		diskPaths[i] = diskPath
	}

	if len(diskPaths) == 0 {
		err = fmt.Errorf("node request must have at least one disk defined to be used as primary disk")

		return
	}

	return
}
