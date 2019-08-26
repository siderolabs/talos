/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package v1alpha1

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/acpi"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/disk"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/kubernetes"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/network"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/security"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/services"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/signal"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/sysctls"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/upgrade"
	userdatatask "github.com/talos-systems/talos/internal/app/machined/internal/phase/userdata"
	"github.com/talos-systems/talos/internal/app/machined/proto"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Sequencer represents the v1alpha1 sequencer.
type Sequencer struct{}

// Boot implements the Sequencer interface.
func (d *Sequencer) Boot() error {
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
			"start system-containerd",
			services.NewStartSystemContainerdTask(),
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
			rootfs.NewMountSharedTask(),
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
			"start services",
			acpi.NewHandlerTask(),
			services.NewStartServicesTask(),
			signal.NewHandlerTask(),
		),
	)

	return phaserunner.Run()
}

// Shutdown implements the Sequencer interface.
func (d *Sequencer) Shutdown() error {
	data := &userdata.UserData{}
	phaserunner, err := phase.NewRunner(data)
	if err != nil {
		return err
	}

	phaserunner.Add(
		phase.NewPhase(
			"stop services",
			services.NewStopServicesTask(),
		),
	)

	return phaserunner.Run()
}

// Upgrade implements the Sequencer interface.
func (d *Sequencer) Upgrade(req *proto.UpgradeRequest) error {
	data, err := userdata.Open(constants.UserDataPath)
	if err != nil {
		return err
	}
	phaserunner, err := phase.NewRunner(data)
	if err != nil {
		return err
	}

	var dev *probe.ProbedBlockDevice
	dev, err = probe.GetDevWithFileSystemLabel(constants.EphemeralPartitionLabel)
	if err != nil {
		return err
	}

	devname := dev.BlockDevice.Device().Name()
	if err := dev.Close(); err != nil {
		return err
	}

	phaserunner.Add(
		phase.NewPhase(
			"cordon and drain node",
			kubernetes.NewCordonAndDrainTask(),
		),
		phase.NewPhase(
			"stop services",
			services.NewStopNonCrucialServicesTask(),
		),
		phase.NewPhase(
			"kill all tasks",
			kubernetes.NewKillKubernetesTasksTask(),
		),
		phase.NewPhase(
			"stop containerd",
			services.NewStopContainerdTask(),
		),
		phase.NewPhase(
			"remove submounts",
			rootfs.NewUnmountOverlayTask(),
			rootfs.NewUnmountPodMountsTask(),
		),
		phase.NewPhase(
			"unmount system disk",
			rootfs.NewUnmountSystemDisksTask(devname),
		),
		phase.NewPhase(
			"reset partition",
			disk.NewResetDiskTask(devname),
		),
		phase.NewPhase(
			"upgrade",
			upgrade.NewUpgradeTask(devname, req),
		),
	)

	return phaserunner.Run()
}
