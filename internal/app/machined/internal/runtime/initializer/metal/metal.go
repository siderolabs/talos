/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package metal

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/pkg/installer"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Metal represents an initializer that performs a full installation to a
// disk.
type Metal struct{}

// Initialize implements the Initializer interface.
func (b *Metal) Initialize(platform platform.Platform, data *userdata.UserData) (err error) {
	var endpoint *string
	if endpoint = kernel.ProcCmdline().Get(constants.KernelParamUserData).First(); endpoint == nil {
		return errors.Errorf("failed to find %s in kernel parameters", constants.KernelParamUserData)
	}

	cmdline := kernel.NewDefaultCmdline()
	cmdline.Append("initrd", filepath.Join("/", "default", "initramfs.xz"))
	cmdline.Append(constants.KernelParamPlatform, strings.ToLower(platform.Name()))
	cmdline.Append(constants.KernelParamUserData, *endpoint)

	if err = cmdline.AppendAll(data.Install.ExtraKernelArgs); err != nil {
		return err
	}

	// Attempt to discover a previous installation
	// An err case should only happen if no partitions
	// with matching labels were found
	var mountpoints *mount.Points
	mountpoints, err = owned.MountPointsFromLabels()
	if err != nil {
		// No previous installation was found, attempt an install
		var i *installer.Installer
		i, err = installer.NewInstaller(cmdline, data)
		if err != nil {
			return err
		}
		if err = i.Install(); err != nil {
			return errors.Wrap(err, "failed to install")
		}

		mountpoints, err = owned.MountPointsFromLabels()
		if err != nil {
			return err
		}
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
