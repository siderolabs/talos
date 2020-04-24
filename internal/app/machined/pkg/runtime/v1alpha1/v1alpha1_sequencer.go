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

// Initialize is the initialize sequence. The primary goals of this sequence is
// to load the config and enforce kernel security requirements.
func (*Sequencer) Initialize(r runtime.Runtime) []runtime.Phase {
	switch r.Platform().Mode() {
	case runtime.ModeContainer:
		return []runtime.Phase{
			{
				WriteRequiredSysctlsForContainer,
				SetupSystemDirectory,
			},
			{
				CreateOSReleaseFile,
			},
			{
				LoadConfig,
			},
		}
	default:
		return []runtime.Phase{
			{
				EnforceKSPPRequirements,
				WriteRequiredSysctls,
				SetupSystemDirectory,
				MountBPFFS,
				MountCgroups,
				MountPseudoFilesystems,
				SetRLimit,
			},
			{
				WriteIMAPolicy,
			},
			{
				CreateEtcNetworkFiles,
				CreateOSReleaseFile,
			},
			{
				SetupDiscoveryNetwork,
			},
			{
				LoadConfig,
			},
			{
				ResetNetwork,
			},
			{
				SetupDiscoveryNetwork,
			},
		}
	}
}

// Install is the install sequence. This sequence should be ran only if an
// installation does not exist.
func (*Sequencer) Install(r runtime.Runtime) []runtime.Phase {
	switch r.Platform().Mode() {
	case runtime.ModeContainer:
		return nil
	default:
		return []runtime.Phase{
			{
				ValidateConfig,
			},
			{
				SetUserEnvVars,
			},
			{
				StartContainerd,
			},
			{
				Install,
			},
			{
				MountBootPartition,
			},
			{
				SaveConfig,
			},
			{
				UnmountBootPartition,
			},
			{
				StopAllServices,
			},
			{
				Reboot,
			},
		}
	}
}

// Boot is the boot sequence. This primary goal if this sequence is apply user
// supplied settings and start the services for the specific machine type.
func (*Sequencer) Boot(r runtime.Runtime) []runtime.Phase {
	switch r.Platform().Mode() {
	case runtime.ModeContainer:
		return []runtime.Phase{
			{
				ValidateConfig,
			},
			{
				SaveConfig,
			},
			{
				SetUserEnvVars,
			},
			{
				StartContainerd,
			},
			{
				PullPlatformMetadata,
			},
			{
				SetupSharedFilesystems,
				SetupVarDirectory,
			},
			{
				WriteUserFiles,
				WriteUserSysctls,
			},
			{
				StartAllServices,
			},
			{
				LabelNodeAsMaster,
			},
		}
	default:
		return []runtime.Phase{
			{
				MountBootPartition,
			},
			{
				ValidateConfig,
			},
			{
				SetUserEnvVars,
			},
			{
				StartContainerd,
			},
			{
				PullPlatformMetadata,
				MountEphermeralPartition,
			},
			{
				VerifyInstallation,
			},
			{
				MountOverlayFilesystems,
				SetupVarDirectory,
			},
			{
				MountUserDisks,
			},
			{
				WriteUserFiles,
				WriteUserSysctls,
			},
			{
				StartAllServices,
			},
			{
				LabelNodeAsMaster,
			},
			{
				UpdateBootloader,
			},
		}
	}
}

// Reboot is the reboot sequence.
func (*Sequencer) Reboot(r runtime.Runtime) []runtime.Phase {
	switch r.Platform().Mode() {
	case runtime.ModeContainer:
		return []runtime.Phase{
			{
				StopAllServices,
			},
			{
				Reboot,
			},
		}
	default:
		return []runtime.Phase{
			{
				StopAllServices,
			},
			{
				UnmountOverlayFilesystems,
				UnmountPodMounts,
			},
			{
				UnmountBootPartition,
				UnmountEphemeralPartition,
			},
			{
				UnmountSystemDiskBindMounts,
			},
			{
				Reboot,
			},
		}
	}
}

// Reset is the reset sequence.
func (*Sequencer) Reset(r runtime.Runtime, in *machine.ResetRequest) []runtime.Phase {
	switch r.Platform().Mode() {
	case runtime.ModeContainer:
		return []runtime.Phase{
			{
				StopAllServices,
			},
			{
				Shutdown,
			},
		}
	default:
		if in.GetGraceful() {
			return []runtime.Phase{
				{
					CordonAndDrainNode,
				},
				{
					LeaveEtcd,
				},
				{
					RemoveAllPods,
				},
				{
					StopAllServices,
				},
				{
					UnmountOverlayFilesystems,
					UnmountPodMounts,
				},
				{
					UnmountBootPartition,
					UnmountEphemeralPartition,
				},
				{
					UnmountSystemDiskBindMounts,
				},
				{
					ResetSystemDisk,
				},
				{
					Reboot,
				},
			}
		}

		return []runtime.Phase{
			{
				StopAllServices,
			},
			{
				UnmountOverlayFilesystems,
				UnmountPodMounts,
			},
			{
				UnmountBootPartition,
				UnmountEphemeralPartition,
			},
			{
				UnmountSystemDiskBindMounts,
			},
			{
				ResetSystemDisk,
			},
			{
				Reboot,
			},
		}
	}
}

// Shutdown is the shutdown sequence.
func (*Sequencer) Shutdown(r runtime.Runtime) []runtime.Phase {
	switch r.Platform().Mode() {
	case runtime.ModeContainer:
		return []runtime.Phase{
			{
				StopAllServices,
			},
			{
				Shutdown,
			},
		}
	default:
		return []runtime.Phase{
			{
				StopAllServices,
			},
			{
				UnmountOverlayFilesystems,
				UnmountPodMounts,
			},
			{
				UnmountBootPartition,
				UnmountEphemeralPartition,
			},
			{
				UnmountSystemDiskBindMounts,
			},
			{
				Shutdown,
			},
		}
	}
}

// Upgrade is the upgrade sequence.
func (*Sequencer) Upgrade(r runtime.Runtime, in *machine.UpgradeRequest) []runtime.Phase {
	switch r.Platform().Mode() {
	case runtime.ModeContainer:
		return nil
	default:
		return []runtime.Phase{
			{
				CordonAndDrainNode,
			},
			{
				LeaveEtcd,
			},
			{
				RemoveAllPods,
			},
			{
				StopServicesForUpgrade,
			},
			{
				UnmountOverlayFilesystems,
				UnmountPodMounts,
			},
			{
				UnmountBootPartition,
				UnmountEphemeralPartition,
			},
			{
				UnmountSystemDiskBindMounts,
			},
			{
				VerifyDiskAvailability,
			},
			{
				Upgrade,
			},
			{
				StopAllServices,
			},
			{
				Reboot,
			},
		}
	}
}
