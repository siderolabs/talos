/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/cgroups"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

const (
	memoryCgroup                  = "memory"
	memoryUseHierarchy            = "memory.use_hierarchy"
	memoryUseHierarchyPermissions = os.FileMode(400)
)

var memoryUseHierarchyContents = []byte(strconv.Itoa(1))

// MountCgroups represents the MountCgroups task.
type MountCgroups struct{}

// NewMountCgroupsTask initializes and returns an MountCgroups task.
func NewMountCgroupsTask() phase.Task {
	return &MountCgroups{}
}

// RuntimeFunc returns the runtime function.
func (task *MountCgroups) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *MountCgroups) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	var mountpoints *mount.Points
	mountpoints, err = cgroups.MountPoints()
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	// See https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
	target := path.Join("/sys/fs/cgroup", memoryCgroup, memoryUseHierarchy)
	if err = ioutil.WriteFile(target, memoryUseHierarchyContents, memoryUseHierarchyPermissions); err != nil {
		return errors.Wrap(err, "failed to enable memory hierarchy support")
	}

	return nil
}
