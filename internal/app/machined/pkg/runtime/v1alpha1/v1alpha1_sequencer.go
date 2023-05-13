// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"strconv"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

// AppendWithDeferredCheck appends a task to the phase list but skips the sequence if `check func()` returns `false` during execution.
func (p PhaseList) AppendWithDeferredCheck(check func() bool, name string, tasks ...runtime.TaskSetupFunc) PhaseList {
	p = append(p, runtime.Phase{
		Name:      name,
		Tasks:     tasks,
		CheckFunc: check,
	})

	return p
}

// AppendList appends an additional PhaseList to the existing one.
func (p PhaseList) AppendList(list PhaseList) PhaseList {
	return append(p, list...)
}

// Initialize is the initialize sequence. The primary goals of this sequence is
// to load the config and enforce kernel security requirements.
func (*Sequencer) Initialize(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint:exhaustive
	case runtime.ModeContainer:
		phases = phases.Append(
			"logger",
			SetupLogger,
		).Append(
			"systemRequirements",
			SetupSystemDirectory,
		).Append(
			"etc",
			CreateSystemCgroups,
			CreateOSReleaseFile,
		).Append(
			"machined",
			StartMachined,
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
			CreateSystemCgroups,
			CreateOSReleaseFile,
		).Append(
			"earlyServices",
			StartUdevd,
			StartMachined,
		).Append(
			"meta",
			ReloadMeta,
		).AppendWithDeferredCheck(
			func() bool {
				disabledStr := procfs.ProcCmdline().Get(constants.KernelParamDashboardDisabled).First()
				disabled, _ := strconv.ParseBool(pointer.SafeDeref(disabledStr)) //nolint:errcheck

				return !disabled
			},
			"dashboard",
			StartDashboard,
		).AppendWithDeferredCheck(
			func() bool {
				wipe := procfs.ProcCmdline().Get(constants.KernelParamWipe).First()

				return pointer.SafeDeref(wipe) != ""
			},
			"wipeDisks",
			ResetSystemDiskPartitions,
		).AppendWithDeferredCheck(
			func() bool {
				return r.State().Machine().Installed()
			},
			"mountSystem",
			MountStatePartition,
		).Append(
			"config",
			LoadConfig,
		).AppendWithDeferredCheck(
			func() bool {
				return r.State().Machine().Installed()
			},
			"unmountSystem",
			UnmountStatePartition,
		)
	}

	return phases
}

// Install is the install sequence.
func (*Sequencer) Install(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint:exhaustive
	case runtime.ModeContainer:
		return nil
	default:
		if !r.State().Machine().Installed() || r.State().Machine().IsInstallStaged() {
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
				"meta",
				ReloadMeta,
			).Append(
				"saveMeta", // saving META here to merge in-memory changes with the on-disk ones from the installer
				FlushMeta,
			).Append(
				"saveStateEncryptionConfig",
				SaveStateEncryptionConfig,
			).Append(
				"mountState",
				MountStatePartition,
			).Append(
				"saveConfig",
				SaveConfig,
			).Append(
				"unmountState",
				UnmountStatePartition,
			).Append(
				"stopEverything",
				StopAllServices,
			).Append(
				"mountBoot",
				MountBootPartition,
			).Append(
				"kexec",
				KexecPrepare,
			).Append(
				"unmountBoot",
				UnmountBootPartition,
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
		"saveStateEncryptionConfig",
		SaveStateEncryptionConfig,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"mountState",
		MountStatePartition,
	).Append(
		"validateConfig",
		ValidateConfig,
	).Append(
		"saveConfig",
		SaveConfig,
	).Append(
		"memorySizeCheck",
		MemorySizeCheck,
	).Append(
		"diskSizeCheck",
		DiskSizeCheck,
	).Append(
		"env",
		SetUserEnvVars,
	).Append(
		"containerd",
		StartContainerd,
	).Append(
		"dbus",
		StartDBus,
	).AppendWhen(
		r.State().Platform().Mode() == runtime.ModeContainer,
		"sharedFilesystems",
		SetupSharedFilesystems,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"ephemeral",
		MountEphemeralPartition,
	).Append(
		"var",
		SetupVarDirectory,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"overlay",
		MountOverlayFilesystems,
	).Append(
		"legacyCleanup",
		CleanupLegacyStaticPodFiles,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer && len(r.Config().Machine().Udev().Rules()) > 0,
		"udevSetup",
		WriteUdevRules,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"userDisks",
		pauseOnFailure(MountUserDisks, constants.FailurePauseTimeout),
	).Append(
		"userSetup",
		pauseOnFailure(WriteUserFiles, constants.FailurePauseTimeout),
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"lvm",
		ActivateLogicalVolumes,
	).Append(
		"startEverything",
		StartAllServices,
	).AppendWhen(
		r.Config().Machine().Type() != machine.TypeWorker && !r.Config().Machine().Kubelet().SkipNodeRegistration(),
		"labelControlPlane",
		LabelNodeAsControlPlane,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer && !r.Config().Machine().Kubelet().SkipNodeRegistration(),
		"uncordon",
		UncordonNode,
	).AppendWhen(
		r.State().Platform().Mode() != runtime.ModeContainer,
		"bootloader",
		UpdateBootloader,
	)

	return phases
}

// Reboot is the reboot sequence.
func (*Sequencer) Reboot(r runtime.Runtime) []runtime.Phase {
	phases := PhaseList{}.Append(
		"cleanup",
		StopAllPods,
	).Append(
		"dbus",
		StopDBus,
	).
		AppendList(stopAllPhaselist(r, true)).
		Append("reboot", Reboot)

	return phases
}

// Reset is the reset sequence.
//
//nolint:gocyclo,cyclop
func (*Sequencer) Reset(r runtime.Runtime, in runtime.ResetOptions) []runtime.Phase {
	phases := PhaseList{}

	// Use kexec if we don't wipe the boot partition.
	withKexec := false
	if len(in.GetSystemDiskTargets()) > 0 {
		withKexec = !bootPartitionInTargets(in.GetSystemDiskTargets())
	}

	var (
		resetUserDisks  bool
		resetSystemDisk bool
	)

	switch in.GetMode() {
	case machineapi.ResetRequest_ALL:
		resetUserDisks = true
		resetSystemDisk = true
	case machineapi.ResetRequest_USER_DISKS:
		resetUserDisks = true
	case machineapi.ResetRequest_SYSTEM_DISK:
		resetSystemDisk = true
	}

	switch r.State().Platform().Mode() { //nolint:exhaustive
	case runtime.ModeContainer:
		phases = phases.AppendList(stopAllPhaselist(r, false)).
			Append(
				"shutdown",
				Shutdown,
			)
	default:
		phases = phases.AppendWhen(
			in.GetGraceful() && !r.Config().Machine().Kubelet().SkipNodeRegistration(),
			"drain",
			taskErrorHandler(logError, CordonAndDrainNode),
		).AppendWhen(
			in.GetGraceful(),
			"cleanup",
			taskErrorHandler(logError, RemoveAllPods),
		).AppendWhen(
			!in.GetGraceful(),
			"cleanup",
			taskErrorHandler(logError, StopAllPods),
		).Append(
			"dbus",
			StopDBus,
		).AppendWhen(
			in.GetGraceful() && (r.Config().Machine().Type() != machine.TypeWorker),
			"leave",
			LeaveEtcd,
		).AppendList(
			phaseListErrorHandler(logError, stopAllPhaselist(r, withKexec)...),
		).Append(
			"forceCleanup",
			ForceCleanup,
		).AppendWhen(
			len(in.GetSystemDiskTargets()) == 0 && resetSystemDisk,
			"reset",
			ResetSystemDisk,
		).AppendWhen(
			len(in.GetSystemDiskTargets()) > 0 && resetSystemDisk,
			"resetSpec",
			ResetSystemDiskSpec,
		).AppendWhen(
			len(in.GetUserDisksToWipe()) > 0 && resetUserDisks,
			"resetUserDisks",
			ResetUserDisks,
		).AppendWhen(
			in.GetReboot(),
			"reboot",
			Reboot,
		).AppendWhen(
			!in.GetReboot(),
			"shutdown",
			Shutdown,
		)
	}

	return phases
}

// Shutdown is the shutdown sequence.
func (*Sequencer) Shutdown(r runtime.Runtime, in *machineapi.ShutdownRequest) []runtime.Phase {
	phases := PhaseList{}.AppendWhen(
		!in.GetForce() && !r.Config().Machine().Kubelet().SkipNodeRegistration(),
		"drain",
		CordonAndDrainNode,
	).Append(
		"cleanup",
		StopAllPods,
	).Append(
		"dbus",
		StopDBus,
	).
		AppendList(stopAllPhaselist(r, false)).
		Append("shutdown", Shutdown)

	return phases
}

// StageUpgrade is the stage upgrade sequence.
func (*Sequencer) StageUpgrade(r runtime.Runtime, in *machineapi.UpgradeRequest) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint:exhaustive
	case runtime.ModeContainer:
		return nil
	default:
		phases = phases.Append(
			"cleanup",
			StopAllPods,
		).Append(
			"dbus",
			StopDBus,
		).AppendWhen(
			!in.GetPreserve() && (r.Config().Machine().Type() != machine.TypeWorker),
			"leave",
			LeaveEtcd,
		).AppendList(
			stopAllPhaselist(r, true),
		).Append(
			"reboot",
			Reboot,
		)
	}

	return phases
}

// MaintenanceUpgrade is the upgrade sequence in maintenance mode.
func (*Sequencer) MaintenanceUpgrade(r runtime.Runtime, _ *machineapi.UpgradeRequest) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint:exhaustive
	case runtime.ModeContainer:
		return nil
	default:
		phases = phases.Append(
			"containerd",
			StartContainerd,
		).Append(
			"verifyDisk",
			VerifyDiskAvailability,
		).Append(
			"upgrade",
			Upgrade,
		).Append(
			"meta",
			ReloadMeta,
		).Append(
			"mountBoot",
			MountBootPartition,
		).Append(
			"kexec",
			KexecPrepare,
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

	return phases
}

// Upgrade is the upgrade sequence.
func (*Sequencer) Upgrade(r runtime.Runtime, in *machineapi.UpgradeRequest) []runtime.Phase {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint:exhaustive
	case runtime.ModeContainer:
		return nil
	default:
		phases = phases.AppendWhen(
			!r.Config().Machine().Kubelet().SkipNodeRegistration(),
			"drain",
			CordonAndDrainNode,
		).AppendWhen(
			!in.GetPreserve(),
			"cleanup",
			RemoveAllPods,
		).AppendWhen(
			in.GetPreserve(),
			"cleanup",
			StopAllPods,
		).Append(
			"dbus",
			StopDBus,
		).AppendWhen(
			!in.GetPreserve() && (r.Config().Machine().Type() != machine.TypeWorker),
			"leave",
			LeaveEtcd,
		).Append(
			"stopServices",
			StopServicesEphemeral,
		).Append(
			"unmountUser",
			UnmountUserDisks,
		).Append(
			"unmount",
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			"unmountBind",
			UnmountSystemDiskBindMounts,
		).Append(
			"unmountSystem",
			UnmountEphemeralPartition,
			UnmountStatePartition,
		).Append(
			"verifyDisk",
			VerifyDiskAvailability,
		).Append(
			"upgrade",
			Upgrade,
		).Append(
			"meta",
			ReloadMeta,
		).Append(
			"mountBoot",
			MountBootPartition,
		).Append(
			"kexec",
			KexecPrepare,
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

	return phases
}

func stopAllPhaselist(r runtime.Runtime, enableKexec bool) PhaseList {
	phases := PhaseList{}

	switch r.State().Platform().Mode() { //nolint:exhaustive
	case runtime.ModeContainer:
		phases = phases.Append(
			"stopEverything",
			StopAllServices,
		)
	default:
		phases = phases.Append(
			"stopServices",
			StopServicesEphemeral,
		).Append(
			"unmountUser",
			UnmountUserDisks,
		).Append(
			"umount",
			UnmountOverlayFilesystems,
			UnmountPodMounts,
		).Append(
			"unmountBind",
			UnmountSystemDiskBindMounts,
		).Append(
			"unmountSystem",
			UnmountEphemeralPartition,
			UnmountStatePartition,
		).AppendWhen(
			enableKexec,
			"mountBoot",
			MountBootPartition,
		).AppendWhen(
			enableKexec,
			"kexec",
			KexecPrepare,
		).AppendWhen(
			enableKexec,
			"unmountBoot",
			UnmountBootPartition,
		).Append(
			"stopEverything",
			StopAllServices,
		)
	}

	return phases
}

func bootPartitionInTargets(targets []runtime.PartitionTarget) bool {
	for _, target := range targets {
		if target.GetLabel() == constants.BootPartitionLabel {
			return true
		}
	}

	return false
}
