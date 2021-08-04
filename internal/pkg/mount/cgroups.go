// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// CGroupMountPoints returns the cgroup mount points.
func CGroupMountPoints() (mountpoints *Points, err error) {
	cgroups := NewMountPoints()
	cgroups.Set("cgroup2", NewMountPoint("cgroup", constants.CgroupMountPath, "cgroup2", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, "nsdelegate,memory_recursiveprot"))

	return cgroups, nil
}
