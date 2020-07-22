// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// Sequencer implements the sequencer interface.
type Sequencer struct{}

// NewSequencer intializes and returns a sequencer.
func NewSequencer() *Sequencer {
	return &Sequencer{}
}

// PhaseList represents a list of phases.
type PhaseList []runtime.Phase

// Append appends a task to the phase list.
func (p PhaseList) Append(name string, tasks ...runtime.TaskSetupFunc) PhaseList {
	p = append(p, runtime.Phase{
		Name:  name,
		Tasks: tasks,
	})

	return p
}

// AppendWhen appends a task to the phase list when `when` is `true`.
func (p PhaseList) AppendWhen(when bool, name string, tasks ...runtime.TaskSetupFunc) PhaseList {
	if when {
		p = p.Append(name, tasks...)
	}

	return p
}

// Initialize is the initialize sequence. The primary goals of this sequence is
// to load the config and enforce kernel security requirements.
func (*Sequencer) Initialize(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint: exhaustive
	case runtime.ModeContainer:
		phases = phases.Append(
			"systemRequirements",
			WriteRequiredSysctlsForContainer,
			SetupSystemDirectory,
		).Append(
			"etc",
			CreateOSReleaseFile,
		).Append(
			"config",
			LoadConfig,
		)
	default:
		phases = phases.Append(
			"logger",
			SetupLogger,
		).Append(
			"systemRequirements",
			EnforceKSPPRequirements,
			WriteRequiredSysctls,
			SetupSystemDirectory,
			MountBPFFS,
			MountCgroups,
			MountPseudoFilesystems,
			SetRLimit,
		).Append(
			"integrity",
			WriteIMAPolicy,
		).Append(
			"etc",
			CreateEtcNetworkFiles,
			CreateOSReleaseFile,
		).Append(
			"discoverNetwork",
			SetupDiscoveryNetwork,
			// We MUST mount the boot partition so that this task can attempt to read
			// the config on disk.
		).AppendWhen(
			r.State().Machine().Installed(),
			"mountBoot",
			MountBootPartition,
		).Append(
			"config",
			LoadConfig,
			// We unmount the boot partition here to simplify subsequent sequences.
			// If we leave it mounted, it becomes tricky trying to figure out if we
			// need to mount the boot partition.
		).AppendWhen(
			r.State().Machine().Installed(),
			"unmountBoot",
			UnmountBootPartition,
		).Append(
			"resetNetwork",
			ResetNetwork,
		).Append(
			"setupNetwork",
			SetupDiscoveryNetwork,
		)
	}

	return phases
}

// Install is the install sequence.
func (*Sequencer) Install(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint: exhaustive
	case runtime.ModeContainer:
		return nil
	default:
		if !r.State().Machine().Installed() {
			phases = phases.Append(
				"validateConfig",
				ValidateConfig,
			).Append(
				"env",
				SetUserEnvVars,
			).Append(
				"containerd",
				StartContainerd,
			).Append(
				"install",
				Install,
			).Append(
				"mountBoot",
				MountBootPartition,
			).Append(
				"saveConfig",
				SaveConfig,
			).Append(
				"unmountBoot",
				UnmountBootPartition,
			).Append(
				"stopEverything",
				StopAllServices,
			).Append(
				"reboot",
				Reboot,
			)
		}
	}

	return phases
}

// Boot is the boot sequence. This primary goal if this sequence is to apply
// user supplied settings and start the services for the specific machine type.
// This sequence should never be reached if an installation is not found.
func (*Sequencer) Boot(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	phases = phases.AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"mountBoot",
		MountBootPartition,
	).Append(
		"validateConfig",
		ValidateConfig,
	).Append(
		"saveConfig",
		SaveConfig,
	).Append(
		"env",
		SetUserEnvVars,
	).Append(
		"containerd",
		StartContainerd,
	).AppendWhen(
		r.State().Platform().Mode() == runtime.ModeContainer,
		"sharedFilesystems",
		SetupSharedFilesystems,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"ephemeral",
		MountEphermeralPartition,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"verifyInstall",
		VerifyInstallation,
	).Append(
		"var",
		SetupVarDirectory,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"overlay",
		MountOverlayFilesystems,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"udevd",
		StartUdevd,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"userDisks",
		MountUserDisks,
	).Append(
		"userSetup",
		WriteUserFiles,
		WriteUserSysctls,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"lvm",
		ActivateLogicalVolumes,
	).Append(
		"startEverything",
		StartAllServices,
	).AppendWhen(
		r.Config().Machine().Type() != runtime.MachineTypeJoin,
		"labelMaster",
		LabelNodeAsMaster,
	).Append(
		"waitForBootstrap",
		WaitForBootkube,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"uncordon",
		UncordonNode,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"bootloader",
		UpdateBootloader,
	)

	return phases
}

// Bootstrap is the bootstrap sequence. This primary goal if this sequence is
// to bootstrap Etcd and Kubernetes.
func (*Sequencer) Bootstrap(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	phases = phases.Append(
		"etcd",
		BootstrapEtcd,
	).Append(
		"kubernetes",
		BootstrapKubernetes,
		LabelNodeAsMaster,
	).Append(
		"initStatus",
		SetInitStatus,
	)

	return phases
}

// Reboot is the reboot sequence.
func (*Sequencer) Reboot(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint: exhaustive
	case runtime.ModeContainer:
		phases = phases.Append(
			"stopEverything",
			StopAllServices,
		).Append(
			"reboot",
			Reboot,
		)
	default:
		phases = phases.Append(
			"stopEverything",
			StopAllServices,
		).Append(
			"umount",
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			"unmountSystem",
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			"unmountBind",
			UnmountSystemDiskBindMounts,
		).Append(
			"reboot",
			Reboot,
		)
	}

	return phases
}

// Recover is the recover sequence.
func (*Sequencer) Recover(r runtime.Runtime, in *machine.RecoverRequest) []runtime.Phase {
	phases := PhaseList{}

	phases = phases.Append("recover", Recover)

	return phases
}

// Reset is the reset sequence.
func (*Sequencer) Reset(r runtime.Runtime, in *machine.ResetRequest) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint: exhaustive
	case runtime.ModeContainer:
		phases = phases.Append(
			"stopEverything",
			StopAllServices,
		).Append(
			"shutdown",
			Shutdown,
		)
	default:
		phases = phases.AppendWhen(
			in.GetGraceful(),
			"drain",
			CordonAndDrainNode,
		).AppendWhen(
			in.GetGraceful() && (r.Config().Machine().Type() != runtime.MachineTypeJoin),
			"leave",
			LeaveEtcd,
		).AppendWhen(
			in.GetGraceful(),
			"cleanup",
			RemoveAllPods,
		).Append(
			"stopEverything",
			StopAllServices,
		).Append(
			"umount",
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			"unmountSystem",
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			"unmountBind",
			UnmountSystemDiskBindMounts,
		).Append(
			"reset",
			ResetSystemDisk,
		).Append(
			"reboot",
			Reboot,
		)
	}

	return phases
}

// Shutdown is the shutdown sequence.
func (*Sequencer) Shutdown(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint: exhaustive
	case runtime.ModeContainer:
		phases = phases.Append(
			"stopEverything",
			StopAllServices,
		).Append(
			"shutdown",
			Shutdown,
		)
	default:
		phases = phases.Append(
			"stopEverything",
			StopAllServices,
		).Append(
			"umount",
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			"unmountSystem",
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			"unmountBind",
			UnmountSystemDiskBindMounts,
		).Append(
			"shutdown",
			Shutdown,
		)
	}

	return phases
}

// Upgrade is the upgrade sequence.
func (*Sequencer) Upgrade(r runtime.Runtime, in *machine.UpgradeRequest) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint: exhaustive
	case runtime.ModeContainer:
		return nil
	default:
		phases = phases.Append(
			"drain",
			CordonAndDrainNode,
		).AppendWhen(
			!in.GetPreserve() && (r.Config().Machine().Type() != runtime.MachineTypeJoin),
			"leave",
			LeaveEtcd,
		).Append(
			"cleanup",
			RemoveAllPods,
		).Append(
			"stopServices",
			StopServicesForUpgrade,
		).Append(
			"unmount",
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			"unmountSystem",
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			"unmountBind",
			UnmountSystemDiskBindMounts,
		).Append(
			"verifyDisk",
			VerifyDiskAvailability,
		).Append(
			"upgrade",
			Upgrade,
		).Append(
			"stopEverything",
			StopAllServices,
		).Append(
			"reboot",
			Reboot,
		)
	}

	return phases
}
