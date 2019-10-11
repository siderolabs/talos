/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"log"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// UnmountPodMounts represents the UnmountPodMounts task.
type UnmountPodMounts struct{}

// NewUnmountPodMountsTask initializes and returns an UnmountPodMounts task.
func NewUnmountPodMountsTask() phase.Task {
	return &UnmountPodMounts{}
}

// TaskFunc returns the runtime function.
func (task *UnmountPodMounts) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *UnmountPodMounts) standard(r runtime.Runtime) (err error) {
	var b []byte

	if b, err = ioutil.ReadFile("/proc/self/mounts"); err != nil {
		return err
	}

	rdr := bytes.NewReader(b)

	scanner := bufio.NewScanner(rdr)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) < 2 {
			continue
		}

		mountpoint := fields[1]
		if strings.HasPrefix(mountpoint, constants.EphemeralMountPoint+"/") {
			log.Printf("unmounting %s\n", mountpoint)

			if err = unix.Unmount(mountpoint, 0); err != nil {
				return errors.Errorf("error unmounting %s: %v", mountpoint, err)
			}
		}
	}

	if err = scanner.Err(); err != nil {
		return err
	}

	return nil
}
