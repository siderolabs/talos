/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/rootfs/mount"

	"golang.org/x/sys/unix"
)

func kmsg(prefix string) (*os.File, error) {
	out, err := os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0666)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open /dev/kmsg")
	}
	log.SetOutput(out)
	log.SetPrefix(prefix + " ")
	log.SetFlags(0)

	return out, nil
}

// nolint: gocyclo
func initram() (err error) {
	var initializer *mount.Initializer
	if initializer, err = mount.NewInitializer(constants.NewRoot); err != nil {
		return err
	}

	// Mount the special devices.
	if err = initializer.InitSpecial(); err != nil {
		return err
	}

	// Setup logging to /dev/kmsg.
	_, err = kmsg("[talos] [initramfs]")
	if err != nil {
		return err
	}

	// Perform the equivalent of switch_root.
	log.Println("mounting the rootfs")
	if err = initializer.Rootfs(); err != nil {
		return err
	}

	// Perform the equivalent of switch_root.
	log.Println("entering the rootfs")
	if err = initializer.Switch(); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := initram(); err != nil {
		panic(errors.Wrap(err, "early boot failed"))
	}

	// We should never reach this point if things are working as intended.
	panic(errors.New("unknown error"))
}
