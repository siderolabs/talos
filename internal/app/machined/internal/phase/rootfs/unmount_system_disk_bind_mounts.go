// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rootfs

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// UnmountSystemDiskBindMounts represents the UnmountSystemDiskBindMounts task.
type UnmountSystemDiskBindMounts struct {
	devname string
}

// NewUnmountSystemDiskBindMountsTask initializes and returns an UnmountSystemDiskBindMounts task.
func NewUnmountSystemDiskBindMountsTask(devname string) phase.Task {
	return &UnmountSystemDiskBindMounts{
		devname: devname,
	}
}

// TaskFunc returns the runtime function.
func (task *UnmountSystemDiskBindMounts) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *UnmountSystemDiskBindMounts) standard(r runtime.Runtime) (err error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return err
	}

	defer f.Close() //nolint: errcheck

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) < 2 {
			continue
		}

		device := fields[0]
		mountpoint := fields[1]

		if strings.HasPrefix(device, task.devname) {
			log.Printf("unmounting %s\n", mountpoint)

			if err = unix.Unmount(mountpoint, 0); err != nil {
				return fmt.Errorf("error unmounting %s: %w", mountpoint, err)
			}
		}
	}

	return scanner.Err()
}
