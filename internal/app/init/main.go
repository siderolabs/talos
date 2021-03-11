// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/switchroot"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/version"
)

func init() {
	// Explicitly disable memory profiling to save around 1.4MiB of memory.
	runtime.MemProfileRate = 0
}

func run() (err error) {
	// Mount the pseudo devices.
	pseudo, err := mount.PseudoMountPoints()
	if err != nil {
		return err
	}

	if err = mount.Mount(pseudo); err != nil {
		return err
	}

	// Setup logging to /dev/kmsg.
	err = kmsg.SetupLogger(nil, "[talos] [initramfs]", nil)
	if err != nil {
		return err
	}

	log.Printf("booting Talos %s", version.Tag)

	// Mount the rootfs.
	log.Println("mounting the rootfs")

	squashfs, err := mount.SquashfsMountPoints(constants.NewRoot)
	if err != nil {
		return err
	}

	if err = mount.Mount(squashfs); err != nil {
		return err
	}

	// Switch into the new rootfs.
	log.Println("entering the rootfs")

	return switchroot.Switch(constants.NewRoot, pseudo)
}

func recovery() {
	// If panic is set in the kernel flags, we'll hang instead of rebooting.
	// But we still allow users to hit CTRL+ALT+DEL to try and restart when they're ready.
	// Listening for these signals also keep us from deadlocking the goroutine.
	if r := recover(); r != nil {
		log.Printf("recovered from: %+v\n", r)

		p := procfs.ProcCmdline().Get(constants.KernelParamPanic).First()
		if p != nil && *p == "0" {
			log.Printf("panic=0 kernel flag found. sleeping forever")

			exitSignal := make(chan os.Signal, 1)
			signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)
			<-exitSignal
		}

		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}
	}

	//nolint:errcheck
	unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
}

func main() {
	defer recovery()

	if err := run(); err != nil {
		panic(fmt.Errorf("early boot failed: %w", err))
	}

	// We should never reach this point if things are working as intended.
	panic(errors.New("unknown error"))
}
