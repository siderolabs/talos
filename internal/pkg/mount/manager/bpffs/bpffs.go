// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bpffs

import (
	"github.com/talos-systems/talos/internal/pkg/mount"
)

// MountPoints returns the cgroup mount points
func MountPoints() (mountpoints *mount.Points, err error) {
	base := "/sys/fs/bpf"
	bpf := mount.NewMountPoints()
	bpf.Set("bpf", mount.NewMountPoint("bpffs", base, "bpf", 0, ""))

	return bpf, nil
}
