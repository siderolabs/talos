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
//
//nolint:gocyclo
func (p *Provisioner) CreateDisks(state *provision.State, nodeReq provision.NodeRequest) (diskPaths []string, err error) {
	const QEMUAlignment = 4 * 1024 * 1024 // 4 MiB, required by QEMU

	diskPaths = make([]string, len(nodeReq.Disks))

	for i, disk := range nodeReq.Disks {
		if disk.Driver == "virtiofs" {
			// virtiofs does not require disk image files
			// skip creating disk files for such disks, but keep the index consistent
			// and add socket path instead
			//
			// however we need to preallocate a file for shm backing store
			diskPaths[i] = state.GetRelativePath(fmt.Sprintf("%s-%d.virtiofs.sock", nodeReq.Name, i))

			// check if we already have a shm file created
			shmFilePath := state.GetShmPath(fmt.Sprintf("shm-%s", nodeReq.Name))

			if _, err = os.Stat(shmFilePath); os.IsNotExist(err) {
				var shmF *os.File

				shmF, err = os.Create(shmFilePath)
				if err != nil {
					return nil, fmt.Errorf("failed to create shm backing file: %w", err)
				}
				defer shmF.Close() //nolint:errcheck

				shmSize := nodeReq.Memory // already in bytes
				if err = shmF.Truncate(shmSize); err != nil {
					return nil, fmt.Errorf("failed to truncate shm backing file: %w", err)
				}

				if err = fallocate.Fallocate(shmF, 0, shmSize); err != nil {
					return nil, fmt.Errorf("failed to preallocate shm backing file: %w", err)
				}
			} else if err != nil {
				return nil, fmt.Errorf("failed to stat shm backing file: %w", err)
			}

			continue
		}

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
