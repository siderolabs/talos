/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package metal

import (
	"strings"

	"github.com/talos-systems/talos/internal/pkg/event"
	installer "github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/pkg/config/machine"
)

// Metal represents an initializer that performs a full installation to a
// disk.
type Metal struct{}

// Initialize implements the Initializer interface.
func (b *Metal) Initialize(platform platform.Platform, install machine.Install) (err error) {
	// Attempt to discover a previous installation
	// An err case should only happen if no partitions
	// with matching labels were found
	var mountpoints *mount.Points
	mountpoints, err = owned.MountPointsFromLabels()
	if err != nil {
		// if install.Image() == "" {
		// 	install.Image() = fmt.Sprintf("%s:%s", constants.DefaultInstallerImageRepository, version.Tag)
		// }
		if err = installer.Install(install.Image(), install.Disk(), strings.ToLower(platform.Name())); err != nil {
			return err
		}
		event.Bus().Notify(event.Event{Type: event.Reboot})
		// Prevent the task from returning to prevent the next phase from
		// running.
		select {}
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
