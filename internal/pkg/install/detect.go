/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"log"
	"os"
	"path/filepath"

	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/mount"
)

// Exists checks if Talos has already been installed to a block device.
// It works by searching for a filesystem with a well known label, and then,
// if the  filesystem exists, checking for the existence of a file.
func Exists(devpath string) (bool, error) {
	var (
		err error
		dev *probe.ProbedBlockDevice
	)

	if dev, err = probe.DevForFileSystemLabel(devpath, constants.BootPartitionLabel); err == nil {
		// nolint: errcheck
		defer dev.Close()
		if dev.SuperBlock != nil {
			mountpoint := mount.NewMountPoint(dev.Path, "/tmp", dev.SuperBlock.Type(), 0, "")
			if err = mount.WithRetry(mountpoint); err != nil {
				return false, err
			}
			defer func() {
				if err = mount.UnWithRetry(mountpoint); err != nil {
					log.Printf("WARNING: failed to unmount %s from /tmp", dev.Path)
				}
			}()
			_, err = os.Stat(filepath.Join("tmp", "installed"))
			switch {
			case err == nil:
				return true, nil
			case os.IsNotExist(err):
				return false, nil
			default:
				return false, err
			}
		}
	}

	return false, nil
}
