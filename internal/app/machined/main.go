/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/event"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/acpi"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/network"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/security"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/services"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/signal"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/sysctls"
	userdatatask "github.com/talos-systems/talos/internal/app/machined/internal/phase/userdata"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/startup"
	"github.com/talos-systems/talos/pkg/userdata"
)

func run() (err error) {
	if err = startup.RandSeed(); err != nil {
		return err
	}

	if err = os.Setenv("PATH", constants.PATH); err != nil {
		return errors.New("error setting PATH")
	}

	data := &userdata.UserData{}
	phaserunner, err := phase.NewRunner(data)
	if err != nil {
		return err
	}

	phaserunner.Add(
		phase.NewPhase(
			"system requirements",
			security.NewSecurityTask(),
			rootfs.NewSystemDirectoryTask(),
			rootfs.NewMountBPFFSTask(),
			rootfs.NewMountCgroupsTask(),
			rootfs.NewMountSubDevicesTask(),
			sysctls.NewSysctlsTask(),
		),
		phase.NewPhase(
			"basic system configuration",
			rootfs.NewNetworkConfigurationTask(),
			rootfs.NewOSReleaseTask(),
		),
		// Break out network setup into a separate phase
		// so we can use the well known resolv.conf location
		// /etc/resolv.conf
		phase.NewPhase(
			"initial network",
			network.NewUserDefinedNetworkTask(),
		),
		phase.NewPhase(
			"userdata",
			userdatatask.NewUserDataTask(),
		),
		phase.NewPhase(
			"mount extra devices",
			userdatatask.NewExtraDevicesTask(),
		),
		phase.NewPhase(
			"user requests",
			userdatatask.NewPKITask(),
			userdatatask.NewExtraEnvVarsTask(),
			userdatatask.NewExtraFilesTask(),
		),
		phase.NewPhase(
			"platform tasks",
			platform.NewPlatformTask(),
		),
		phase.NewPhase(
			"installation verification",
			rootfs.NewCheckInstallTask(),
		),
		phase.NewPhase(
			"overlay mounts",
			rootfs.NewMountOverlayTask(),
		),
		phase.NewPhase(
			"setup /var",
			rootfs.NewVarDirectoriesTask(),
		),
		phase.NewPhase(
			"save userdata",
			userdatatask.NewSaveUserDataTask(),
		),
		phase.NewPhase(
			"service setup",
			acpi.NewHandlerTask(),
			services.NewServicesTask(),
			signal.NewHandlerTask(),
		),
	)

	if err = phaserunner.Run(); err != nil {
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
}

func sync() {
	syncdone := make(chan struct{})

	go func() {
		defer close(syncdone)
		unix.Sync()
	}()

	log.Printf("waiting for sync...")

	for i := 29; i >= 0; i-- {
		select {
		case <-syncdone:
			log.Printf("sync done")
			return
		case <-time.After(time.Second):
		}
		if i != 0 {
			log.Printf("waiting %d more seconds for sync to finish", i)
		}
	}

	log.Printf("sync hasn't completed in time, aborting...")
}

var rebootFlag = unix.LINUX_REBOOT_CMD_RESTART

func reboot() {
	// See http://man7.org/linux/man-pages/man2/reboot.2.html.
	sync()

	if unix.Reboot(rebootFlag) == nil {
		select {}
	}
}

func main() {
	// This is main entrypoint into machined execution, control is passed here from init after switch root.
	//
	// When machined terminates either on normal shutdown (reboot, poweroff), or due to panic, control
	// goes through recovery() and reboot() functions below, which finalize node state - sync buffers,
	// initiate poweroff or reboot. Also on shutdown, other deferred function are called, for example
	// services are gracefully shutdown.

	defer reboot()

	// on any return from init.main(), initiate host reboot or shutdown
	// handle any panics in the main goroutine, and proceed to reboot() above
	defer recovery()

	// subscribe for events
	events := make(chan event.Type, 5) // provide some buffer to avoid blocking the bus
	event.Bus().Subscribe(events)
	defer event.Bus().Unsubscribe(events)

	// run startup phases
	if err := run(); err != nil {
		panic(errors.Wrap(err, "boot failed"))
	}

	// start services
	system.Services(nil).StartAll()
	defer system.Services(nil).Shutdown()

	// wait for events
	for {
		switch <-events {
		case event.Reboot:
			return
		case event.Shutdown:
			rebootFlag = unix.LINUX_REBOOT_CMD_POWER_OFF
			return
		case event.Upgrade:
			// TODO: not implemented yet
		}
	}
}
