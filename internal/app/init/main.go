/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/squashfs"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/virtual"
	"github.com/talos-systems/talos/internal/pkg/mount/switchroot"
	"github.com/talos-systems/talos/pkg/constants"
)

// nolint: gocyclo
func run() (err error) {
	// Mount the virtual devices.
	mountpoints, err := virtual.MountPoints()
	if err != nil {
		return err
	}
	virtual := manager.NewManager(mountpoints)
	if err = virtual.MountAll(); err != nil {
		return err
	}

	// Setup logging to /dev/kmsg.
	_, err = kmsg.Setup("[talos] [initramfs]")
	if err != nil {
		return err
	}

	// Mount the rootfs.
	log.Println("mounting the rootfs")
	mountpoints, err = squashfs.MountPoints(constants.NewRoot)
	if err != nil {
		return err
	}
	squashfs := manager.NewManager(mountpoints)
	if err = squashfs.MountAll(); err != nil {
		return err
	}

	// Switch into the new rootfs.
	log.Println("entering the rootfs")
	if err = switchroot.Switch(constants.NewRoot, virtual); err != nil {
		return err
	}

	return nil
}

func recovery() {
	if r := recover(); r != nil {
		log.Printf("recovered from: %+v\n", r)
		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}
	}

	// nolint: errcheck
	unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
}

func main() {
	defer recovery()

	if err := run(); err != nil {
		panic(errors.Wrap(err, "early boot failed"))
	}

	// We should never reach this point if things are working as intended.
	panic(errors.New("unknown error"))
}
