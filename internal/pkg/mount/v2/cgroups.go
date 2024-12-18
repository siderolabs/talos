// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CGroupMountPoints returns the cgroup mount points.
func CGroupMountPoints() Points {
	return Points{
		NewPoint("cgroup", constants.CgroupMountPath, "cgroup2", WithFlags(unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME), WithData("nsdelegate,memory_recursiveprot")),
	}
}
