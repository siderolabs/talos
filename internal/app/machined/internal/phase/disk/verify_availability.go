// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disk

import (
	"errors"
	"io"
	"log"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/retry"
)

// VerifyDiskAvailability represents the task for verifying that the system
// disk is not in use.
type VerifyDiskAvailability struct {
	devname string
}

// NewVerifyDiskAvailabilityTask initializes and returns a
// VerifyDiskAvailability task.
func NewVerifyDiskAvailabilityTask(devname string) phase.Task {
	return &VerifyDiskAvailability{
		devname: devname,
	}
}

// TaskFunc returns the runtime function.
func (task *VerifyDiskAvailability) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return func(r runtime.Runtime) error {
		return task.standard()
	}
}

func (task *VerifyDiskAvailability) standard() (err error) {
	mountsReported := false

	return retry.Constant(3*time.Minute, retry.WithUnits(500*time.Millisecond)).Retry(func() error {
		if isBusy(task.devname) {
			if !mountsReported {
				// if disk is busy, report mounts for debugging purposes but just once
				// otherwise console might be flooded with messages
				dumpMounts()
				mountsReported = true
			}

			return retry.ExpectedError(errors.New("system disk in use"))
		}

		return nil
	})
}

func isBusy(devname string) bool {
	fd, errno := unix.Open(devname, unix.O_RDONLY|unix.O_EXCL|unix.O_CLOEXEC, 0)

	// nolint: errcheck
	defer unix.Close(fd)

	return errno == unix.EBUSY
}

func dumpMounts() {
	mounts, err := os.Open("/proc/mounts")
	if err != nil {
		log.Printf("failed to read mounts: %s", err)
		return
	}

	defer mounts.Close() //nolint: errcheck

	log.Printf("contents of /proc/mounts:")

	_, _ = io.Copy(log.Writer(), mounts) //nolint: errcheck
}
