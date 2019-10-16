/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/cgroups"
	"github.com/talos-systems/talos/internal/pkg/runtime"
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

// TaskFunc returns the runtime function.
func (task *MountCgroups) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *MountCgroups) runtime(r runtime.Runtime) (err error) {
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
		return fmt.Errorf("failed to enable memory hierarchy support: %w", err)
	}

	return nil
}
