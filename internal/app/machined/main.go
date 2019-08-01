/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/api"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/network"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/security"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/services"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/sysctls"
	userdatatask "github.com/talos-systems/talos/internal/app/machined/internal/phase/userdata"
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
			rootfs.NewMountCgroupsTask(),
			rootfs.NewMountSubDevicesTask(),
			sysctls.NewSysctlsTask(),
		),
		phase.NewPhase(
			"basic system configuration",
			rootfs.NewNetworkConfigurationTask(),
			rootfs.NewOSReleaseTask(),
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
			network.NewUserDefinedNetworkTask(),
			userdatatask.NewExtraEnvVarsTask(),
			userdatatask.NewExtraFilesTask(),
		),
		phase.NewPhase(
			"platform tasks",
			platform.NewPlatformTask(),
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
			api.NewAPITask(),
			services.NewServicesTask(),
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

func main() {
	// TODO: this comment is outdated
	// This is main entrypoint into init() execution, after kernel boot control is passsed
	// to this function.
	//
	// When initram() finishes, it execs into itself with -switch-root flag, so control is passed
	// once again into this function.
	//
	// When init() terminates either on normal shutdown (reboot, poweroff), or due to panic, control
	// goes through recovery() and reboot() functions below, which finalize node state - sync buffers,
	// initiate poweroff or reboot. Also on shutdown, other deferred function are called, for example
	// services are gracefully shutdown.

	// on any return from init.main(), initiate host reboot or shutdown
	// handle any panics in the main goroutine, and proceed to reboot() above
	defer recovery()

	if err := run(); err != nil {
		panic(errors.Wrap(err, "boot failed"))
	}

	select {}
}
