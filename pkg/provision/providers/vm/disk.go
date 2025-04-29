// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"errors"
	"fmt"
	"os"

	"github.com/detailyang/go-fallocate"

	"github.com/siderolabs/talos/pkg/provision"
)

// UserDiskName returns disk device path.
func (p *Provisioner) UserDiskName(index int) string {
	// the disk IDs are assigned in the following way:
	// * ata-QEMU_HARDDISK_QM00001
	// * ata-QEMU_HARDDISK_QM00003
	// * ata-QEMU_HARDDISK_QM00005
	return fmt.Sprintf("/dev/disk/by-id/ata-QEMU_HARDDISK_QM%05d", (index-1)*2+1)
}

// CreateDisks creates empty disk files for each disk.
func (p *Provisioner) CreateDisks(state *State, nodeReq provision.NodeRequest) (diskPaths []string, err error) {
	const QEMUAlignment = 4 * 1024 * 1024 // 4 MiB, required by QEMU

	diskPaths = make([]string, len(nodeReq.Disks))

	for i, disk := range nodeReq.Disks {
		diskPath := state.GetRelativePath(fmt.Sprintf("%s-%d.disk", nodeReq.Name, i))
		diskSize := (disk.Size + QEMUAlignment - 1) / QEMUAlignment * QEMUAlignment

		var diskF *os.File

		diskF, err = os.Create(diskPath)
		if err != nil {
			return nil, err
		}

		defer diskF.Close() //nolint:errcheck

		if err = diskF.Truncate(int64(diskSize)); err != nil {
			return nil, err
		}

		if !disk.SkipPreallocate {
			if err = fallocate.Fallocate(diskF, 0, int64(disk.Size)); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: failed to preallocate disk space for %q (size %d): %s", diskPath, diskSize, err)
			}
		}

		diskPaths[i] = diskPath
	}

	if len(diskPaths) == 0 {
		return nil, errors.New("node request must have at least one disk defined to be used as primary disk")
	}

	return diskPaths, nil
}
