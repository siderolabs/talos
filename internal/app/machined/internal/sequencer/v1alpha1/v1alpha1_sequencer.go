// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/acpi"
	configtask "github.com/talos-systems/talos/internal/app/machined/internal/phase/config"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/disk"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/kubernetes"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/limits"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/network"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/security"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/services"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/signal"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/sysctls"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/upgrade"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
)

// Sequencer represents the v1alpha1 sequencer.
type Sequencer struct{}

// Boot implements the Sequencer interface.
func (d *Sequencer) Boot() error {
	phaserunner, err := phase.NewRunner(nil, runtime.Boot)
	if err != nil {
		return err
	}

	cfgBytes := []byte{}

	phaserunner.Add(
		phase.NewPhase(
			"system requirements",
			security.NewSecurityTask(),
			rootfs.NewSystemDirectoryTask(),
			rootfs.NewMountBPFFSTask(),
			rootfs.NewMountCgroupsTask(),
			rootfs.NewMountSubDevicesTask(),
			sysctls.NewSysctlsTask(),
			limits.NewFileLimitTask(),
		),
		phase.NewPhase(
			"configure Integrity Measurement Architecture",
			security.NewIMATask(),
		),
		phase.NewPhase(
			"basic system configuration",
			rootfs.NewNetworkConfigurationTask(),
			rootfs.NewOSReleaseTask(),
		),
		phase.NewPhase(
			"discover network",
			network.NewInitialNetworkSetupTask(),
		),
		phase.NewPhase(
			"mount /boot as ro",
			rootfs.NewMountSystemDisksTask(constants.BootPartitionLabel, mount.WithReadOnly(true)),
		),
		phase.NewPhase(
			"config",
			configtask.NewConfigTask(&cfgBytes),
		),
	)

	if err = phaserunner.Run(); err != nil {
		return err
	}

	cfg, err := config.NewFromBytes(cfgBytes)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	phaserunner, err = phase.NewRunner(cfg, runtime.Boot)
	if err != nil {
		return err
	}

	phaserunner.Add(
		phase.NewPhase(
			"config validation",
			rootfs.NewValidateConfigTask(),
		),
		phase.NewPhase(
			"network reset",
			network.NewResetNetworkTask(),
			configtask.NewExtraEnvVarsTask(),
		),
		phase.NewPhase(
			"initial network",
			network.NewInitialNetworkSetupTask(),
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
			"unmount /boot",
			rootfs.NewUnmountSystemDisksTask(constants.BootPartitionLabel),
		),
		phase.NewPhase(
			"mount /boot as rw",
			rootfs.NewMountSystemDisksTask(constants.BootPartitionLabel),
		),
		phase.NewPhase(
			"save config",
			configtask.NewSaveConfigTask(),
		),
		phase.NewPhase(
			"unmount /boot",
			rootfs.NewUnmountSystemDisksTask(constants.BootPartitionLabel),
		),
		phase.NewPhase(
			"mount /boot as ro",
			rootfs.NewMountSystemDisksTask(constants.BootPartitionLabel, mount.WithReadOnly(true)),
		),
		phase.NewPhase(
			"mount extra disks",
			configtask.NewExtraDisksTask(),
		),
		phase.NewPhase(
			"user requests",
			configtask.NewExtraFilesTask(),
			configtask.NewSysctlsTask(),
		),
		phase.NewPhase(
			"start services",
			acpi.NewHandlerTask(),
			services.NewStartServicesTask(),
			signal.NewHandlerTask(),
		),
		phase.NewPhase(
			"post startup tasks",
			services.NewLabelNodeAsMasterTask(),
		),
	)

	return phaserunner.Run()
}

// Shutdown implements the Sequencer interface.
func (d *Sequencer) Shutdown() error {
	config, err := config.NewFromFile(constants.ConfigPath)
	if err != nil {
		return err
	}

	phaserunner, err := phase.NewRunner(config, runtime.Shutdown)
	if err != nil {
		return err
	}

	phaserunner.Add(
		phase.NewPhase(
			"stop services",
			services.NewStopServicesTask(false),
		),
	)

	return phaserunner.Run()
}

// Upgrade implements the Sequencer interface.
func (d *Sequencer) Upgrade(req *machineapi.UpgradeRequest) error {
	config, err := config.NewFromFile(constants.ConfigPath)
	if err != nil {
		return err
	}

	phaserunner, err := phase.NewRunner(config, runtime.Upgrade)
	if err != nil {
		return err
	}

	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(constants.EphemeralPartitionLabel)
	if err != nil {
		return err
	}

	devname := dev.Device().Name()

	if err := dev.Close(); err != nil {
		return err
	}

	phaserunner.Add(
		phase.NewPhase(
			"cordon and drain node",
			kubernetes.NewCordonAndDrainTask(),
		),
		phase.NewPhase(
			"handle control plane requirements",
			upgrade.NewLeaveEtcdTask(),
		),
		phase.NewPhase(
			"remove all pods",
			kubernetes.NewRemoveAllPodsTask(),
		),
		phase.NewPhase(
			"stop services",
			services.NewStopServicesTask(true),
		),
		phase.NewPhase(
			"unmount system disk submounts",
			rootfs.NewUnmountOverlayTask(),
			rootfs.NewUnmountPodMountsTask(),
		),
		phase.NewPhase(
			"unmount system disk",
			rootfs.NewUnmountSystemDisksTask(constants.EphemeralPartitionLabel),
		),
		phase.NewPhase(
			"unmount system disk bind mounts",
			rootfs.NewUnmountSystemDiskBindMountsTask(devname),
		),
		phase.NewPhase(
			"verify system disk not in use",
			disk.NewVerifyDiskAvailabilityTask(devname),
		),
		phase.NewPhase(
			"reset system disk",
			disk.NewResetSystemDiskTask(devname),
		),
		phase.NewPhase(
			"upgrade",
			upgrade.NewUpgradeTask(devname, req),
		),
	)

	return phaserunner.Run()
}

// Reset implements the Sequencer interface.
func (d *Sequencer) Reset(req *machineapi.ResetRequest) error {
	config, err := config.NewFromFile(constants.ConfigPath)
	if err != nil {
		return err
	}

	phaserunner, err := phase.NewRunner(config, runtime.Reset)
	if err != nil {
		return err
	}

	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(constants.EphemeralPartitionLabel)
	if err != nil {
		return err
	}

	devname := dev.Device().Name()

	if err := dev.Close(); err != nil {
		return err
	}

	if req.GetGraceful() {
		phaserunner.Add(
			phase.NewPhase(
				"cordon and drain node",
				kubernetes.NewCordonAndDrainTask(),
			),
			phase.NewPhase(
				"handle control plane requirements",
				upgrade.NewLeaveEtcdTask(),
			),
			phase.NewPhase(
				"remove all pods",
				kubernetes.NewRemoveAllPodsTask(),
			),
		)
	}

	phaserunner.Add(
		phase.NewPhase(
			"stop services",
			services.NewStopServicesTask(true),
		),
		phase.NewPhase(
			"unmount system disk submounts",
			rootfs.NewUnmountOverlayTask(),
			rootfs.NewUnmountPodMountsTask(),
		),
		phase.NewPhase(
			"unmount system disk",
			rootfs.NewUnmountSystemDisksTask(constants.EphemeralPartitionLabel),
		),
		phase.NewPhase(
			"reset system disk",
			disk.NewResetSystemDiskTask(devname),
		),
	)

	return phaserunner.Run()
}
