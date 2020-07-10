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
func (p PhaseList) Append(tasks ...runtime.TaskSetupFunc) PhaseList {
	p = append(p, tasks)

	return p
}

// AppendWhen appends a task to the phase list when `when` is `true`.
func (p PhaseList) AppendWhen(when bool, tasks ...runtime.TaskSetupFunc) PhaseList {
	if when {
		p = p.Append(tasks...)
	}

	return p
}

// Initialize is the initialize sequence. The primary goals of this sequence is
// to load the config and enforce kernel security requirements.
func (*Sequencer) Initialize(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() {
	case runtime.ModeContainer:
		phases = phases.Append(
			WriteRequiredSysctlsForContainer,
			SetupSystemDirectory,
		).Append(
			CreateOSReleaseFile,
		).Append(
			LoadConfig,
		)
	default:
		phases = phases.Append(
			SetupLogger,
		).Append(
			EnforceKSPPRequirements,
			WriteRequiredSysctls,
			SetupSystemDirectory,
			MountBPFFS,
			MountCgroups,
			MountPseudoFilesystems,
			SetRLimit,
		).Append(
			WriteIMAPolicy,
		).Append(
			CreateEtcNetworkFiles,
			CreateOSReleaseFile,
		).Append(
			SetupDiscoveryNetwork,
			// We MUST mount the boot partition so that this task can attempt to read
			// the config on disk.
		).AppendWhen(
			r.State().Machine().Installed(),
			MountBootPartition,
		).Append(
			LoadConfig,
			// We unmount the boot partition here to simplify subsequent sequences.
			// If we leave it mounted, it becomes tricky trying to figure out if we
			// need to mount the boot partition.
		).AppendWhen(
			r.State().Machine().Installed(),
			UnmountBootPartition,
		).Append(
			ResetNetwork,
		).Append(
			SetupDiscoveryNetwork,
		)
	}

	return phases
}

// Install is the install sequence.
func (*Sequencer) Install(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() {
	case runtime.ModeContainer:
		return nil
	default:
		if !r.State().Machine().Installed() {
			phases = phases.Append(
				ValidateConfig,
			).Append(
				SetUserEnvVars,
			).Append(
				StartContainerd,
			).Append(
				Install,
			).Append(
				MountBootPartition,
			).Append(
				SaveConfig,
			).Append(
				UnmountBootPartition,
			).Append(
				StopAllServices,
			).Append(
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
		MountBootPartition,
	).Append(
		ValidateConfig,
	).Append(
		SaveConfig,
	).Append(
		SetUserEnvVars,
	).Append(
		StartContainerd,
	).AppendWhen(
		r.State().Platform().Mode() == runtime.ModeContainer,
		SetupSharedFilesystems,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		MountEphermeralPartition,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		VerifyInstallation,
	).Append(
		SetupVarDirectory,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		MountOverlayFilesystems,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		StartUdevd,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		MountUserDisks,
	).Append(
		WriteUserFiles,
		WriteUserSysctls,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		ActivateLogicalVolumes,
	).Append(
		StartAllServices,
	).AppendWhen(
		r.Config().Machine().Type() != runtime.MachineTypeJoin,
		LabelNodeAsMaster,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		UncordonNode,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		UpdateBootloader,
	)

	return phases
}

// Bootstrap is the bootstrap sequence. This primary goal if this sequence is
// to bootstrap Etcd and Kubernetes.
func (*Sequencer) Bootstrap(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	phases = phases.Append(
		BootstrapEtcd,
	).Append(
		BootstrapKubernetes,
		LabelNodeAsMaster,
	).Append(
		SetInitStatus,
	)

	return phases
}

// Reboot is the reboot sequence.
func (*Sequencer) Reboot(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() {
	case runtime.ModeContainer:
		phases = phases.Append(
			StopAllServices,
		).Append(
			Reboot,
		)
	default:
		phases = phases.Append(
			StopAllServices,
		).Append(
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			UnmountSystemDiskBindMounts,
		).Append(
			Reboot,
		)
	}

	return phases
}

// Recover is the recover sequence.
func (*Sequencer) Recover(r runtime.Runtime, in *machine.RecoverRequest) []runtime.Phase {
	phases := PhaseList{}

	phases = phases.Append(Recover)

	return phases
}

// Reset is the reset sequence.
func (*Sequencer) Reset(r runtime.Runtime, in *machine.ResetRequest) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() {
	case runtime.ModeContainer:
		phases = phases.Append(
			StopAllServices,
		).Append(
			Shutdown,
		)
	default:
		phases = phases.AppendWhen(
			in.GetGraceful(),
			CordonAndDrainNode,
		).AppendWhen(
			in.GetGraceful() && (r.Config().Machine().Type() != runtime.MachineTypeJoin),
			LeaveEtcd,
		).AppendWhen(
			in.GetGraceful(),
			RemoveAllPods,
		).Append(
			StopAllServices,
		).Append(
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			UnmountSystemDiskBindMounts,
		).Append(
			ResetSystemDisk,
		).Append(
			Reboot,
		)
	}

	return phases
}

// Shutdown is the shutdown sequence.
func (*Sequencer) Shutdown(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() {
	case runtime.ModeContainer:
		phases = phases.Append(
			StopAllServices,
		).Append(
			Shutdown,
		)
	default:
		phases = phases.Append(
			StopAllServices,
		).Append(
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			UnmountSystemDiskBindMounts,
		).Append(
			Shutdown,
		)
	}

	return phases
}

// Upgrade is the upgrade sequence.
func (*Sequencer) Upgrade(r runtime.Runtime, in *machine.UpgradeRequest) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() {
	case runtime.ModeContainer:
		return nil
	default:
		phases = phases.Append(
			CordonAndDrainNode,
		).AppendWhen(
			!in.GetPreserve() && (r.Config().Machine().Type() != runtime.MachineTypeJoin),
			LeaveEtcd,
		).Append(
			RemoveAllPods,
		).Append(
			StopServicesForUpgrade,
		).Append(
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			UnmountBootPartition,
			UnmountEphemeralPartition,
		).Append(
			UnmountSystemDiskBindMounts,
		).Append(
			VerifyDiskAvailability,
		).Append(
			Upgrade,
		).Append(
			StopAllServices,
		).Append(
			Reboot,
		)
	}

	return phases
}
