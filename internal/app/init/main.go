/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/internal/platform"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/network"
	"github.com/talos-systems/talos/internal/pkg/rootfs"
	"github.com/talos-systems/talos/internal/pkg/rootfs/mount"
	"github.com/talos-systems/talos/internal/pkg/security/kspp"
	"github.com/talos-systems/talos/pkg/userdata"

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
	// Enforce KSPP kernel parameters.
	log.Println("checking for KSPP kernel parameters")
	if err = kspp.EnforceKSPPKernelParameters(); err != nil {
		return err
	}
	// Setup hostname if provided.
	var hostname *string
	if hostname = kernel.Cmdline().Get(constants.KernelParamHostname).First(); hostname != nil {
		log.Println("setting hostname")
		if err = unix.Sethostname([]byte(*hostname)); err != nil {
			return err
		}
		log.Printf("hostname is: %s", *hostname)
	}
	// Discover the platform.
	log.Println("discovering the platform")
	var p platform.Platform
	if p, err = platform.NewPlatform(); err != nil {
		return err
	}
	log.Printf("platform is: %s", p.Name())
	// Setup basic network.
	if err = network.InitNetwork(); err != nil {
		return err
	}
	// Retrieve the user data.
	var data *userdata.UserData
	log.Printf("retrieving user data")
	if data, err = p.UserData(); err != nil {
		log.Printf("encountered error retrieving userdata: %v", err)
		return err
	}
	// Setup custom network.
	if err = network.SetupNetwork(data); err != nil {
		return err
	}
	// Perform any tasks required by a particular platform.
	log.Printf("performing platform specific tasks")
	if err = p.Prepare(data); err != nil {
		return err
	}
	// Mount the owned partitions.
	log.Printf("mounting the partitions")
	if err = initializer.InitOwned(); err != nil {
		return err
	}
	// Install handles additional system setup
	if err = p.Install(data); err != nil {
		return err
	}
	// Prepare the necessary files in the rootfs.
	log.Println("preparing the root filesystem")
	if err = rootfs.Prepare(constants.NewRoot, false, data); err != nil {
		return err
	}
	// Perform the equivalent of switch_root.
	log.Println("entering the new root")
	if err = initializer.Switch(); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		panic(errors.New("error setting PATH"))
	}

	if err := initram(); err != nil {
		panic(errors.Wrap(err, "early boot failed"))
	}

	// We should never reach this point if things are working as intended.
	panic(errors.New("unknown error"))
}
