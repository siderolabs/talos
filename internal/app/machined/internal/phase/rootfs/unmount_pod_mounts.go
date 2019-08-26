/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"bufio"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
	"golang.org/x/sys/unix"
)

// UnmountPodMounts represents the UnmountPodMounts task.
type UnmountPodMounts struct{}

// NewUnmountPodMountsTask initializes and returns an UnmountPodMounts task.
func NewUnmountPodMountsTask() phase.Task {
	return &UnmountPodMounts{}
}

// RuntimeFunc returns the runtime function.
func (task *UnmountPodMounts) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *UnmountPodMounts) standard(platform platform.Platform, data *userdata.UserData) (err error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) < 2 {
			continue
		}

		mountpoint := fields[1]
		if strings.HasPrefix(mountpoint, constants.EphemeralMountPoint+"/") {
			if err := unix.Unmount(mountpoint, 0); err != nil {
				return errors.Errorf("error creating overlay mount to %s: %v", mountpoint, err)
			}
		}
	}

	return nil
}
