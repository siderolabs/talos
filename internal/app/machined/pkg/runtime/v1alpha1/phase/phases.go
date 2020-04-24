// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package phase

import (
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/phase/task"
	"github.com/talos-systems/talos/pkg/constants"
)

// Security represents a phase.
type Security struct{}

// Tasks implments the phase interface.
func (*Security) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.EnforceKSPPRequirements{},
		&task.WriteRequiredSysctls{},
		&task.SetupSystemDirectory{},
		&task.MountBPFFS{},
		&task.MountCgroups{},
		&task.MountSubDevices{},
		&task.SetFileLimit{},
	}
}

// IMA represents a phase.
type IMA struct{}

// Tasks implments the phase interface.
func (*IMA) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.WriteIMAPolicy{},
	}
}

// ETC represents a phase.
type ETC struct{}

// Tasks implments the phase interface.
func (*ETC) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.CreateEtcNetworkFiles{},
		&task.CreatOSReleaseFile{},
	}
}

// DiscoveryNetwork represents a phase.
type DiscoveryNetwork struct{}

// Tasks implments the phase interface.
func (*DiscoveryNetwork) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.SetupDiscoveryNetwork{},
	}
}

// MountBootPartition represents a phase.
type MountBootPartition struct{}

// Tasks implments the phase interface.
func (*MountBootPartition) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.MountSystemDisk{Label: constants.BootPartitionLabel},
	}
}

// SaveConfig represents a phase.
type SaveConfig struct{}

// Tasks implments the phase interface.
func (*SaveConfig) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.SaveConfig{},
	}
}

// ValidateConfig represents a phase.
type ValidateConfig struct{}

// Tasks implments the phase interface.
func (*ValidateConfig) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.ValidateConfig{},
	}
}

// ResetNetwork represents a phase.
type ResetNetwork struct{}

// Tasks implments the phase interface.
func (*ResetNetwork) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.ResetNetwork{},
	}
}

// SetUserEnvVars represents a phase.
type SetUserEnvVars struct{}

// Tasks implments the phase interface.
func (*SetUserEnvVars) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.SetUserEnvVars{},
	}
}

// StartStage1SystemServices represents a phase.
type StartStage1SystemServices struct{}

// Tasks implments the phase interface.
func (*StartStage1SystemServices) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.StartStage1SystemServices{},
	}
}

// StartStage2SystemServices represents a phase.
type StartStage2SystemServices struct{}

// Tasks implments the phase interface.
func (*StartStage2SystemServices) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.StartStage2SystemServices{},
	}
}

// InitializePlatform represents a phase.
type InitializePlatform struct{}

// Tasks implments the phase interface.
func (*InitializePlatform) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.InitializePlatform{},
	}
}

// VerifyInstallation represents a phase.
type VerifyInstallation struct{}

// Tasks implments the phase interface.
func (*VerifyInstallation) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.VerifyInstallation{},
	}
}

// SetupFilesystems represents a phase.
type SetupFilesystems struct{}

// Tasks implments the phase interface.
func (*SetupFilesystems) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.MountOverlayFilesystems{},
		&task.MountAsShared{},
		&task.SetupVarDirectory{},
	}
}

// MountUserDisks represents a phase.
type MountUserDisks struct{}

// Tasks implments the phase interface.
func (*MountUserDisks) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.MountUserDisks{},
	}
}

// UserRequests represents a phase.
type UserRequests struct{}

// Tasks implments the phase interface.
func (*UserRequests) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.WriteUserFiles{},
		&task.WriteUserSysctls{},
	}
}

// StartOrchestrationServices represents a phase.
type StartOrchestrationServices struct{}

// Tasks implments the phase interface.
func (*StartOrchestrationServices) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.StartOrchestrationServices{},
	}
}

// LabelNode represents a phase.
type LabelNode struct{}

// Tasks implments the phase interface.
func (*LabelNode) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.LabelNodeAsMaster{},
	}
}

// UpdateBootloader represents a phase.
type UpdateBootloader struct{}

// Tasks implments the phase interface.
func (*UpdateBootloader) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.UpdateBootloader{},
	}
}

// StopServices represents a phase.
type StopServices struct{}

// Tasks implments the phase interface.
func (*StopServices) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.StopServices{},
	}
}

// TeardownFilesystems represents a phase.
type TeardownFilesystems struct{}

// Tasks implments the phase interface.
func (*TeardownFilesystems) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.UnmountOverlayFilesystems{},
		&task.UnmountPodMounts{},
	}
}

// UnmountSystemDisks represents a phase.
type UnmountSystemDisks struct{}

// Tasks implments the phase interface.
func (*UnmountSystemDisks) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.UnmountSystemDisk{Label: constants.BootPartitionLabel},
		&task.UnmountSystemDisk{Label: constants.EphemeralPartitionLabel},
	}
}

// UnmountSystemDiskBindMounts represents a phase.
type UnmountSystemDiskBindMounts struct{}

// Tasks implments the phase interface.
func (*UnmountSystemDiskBindMounts) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.UnmountSystemDiskBindMounts{},
	}
}

// CordonAndDrainNode represents a phase.
type CordonAndDrainNode struct{}

// Tasks implments the phase interface.
func (*CordonAndDrainNode) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.CordonAndDrainNode{},
	}
}

// LeaveEtcd represents a phase.
type LeaveEtcd struct {
	Preserve bool
}

// Tasks implments the phase interface.
func (p *LeaveEtcd) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.LeaveEtcd{Preserve: p.Preserve},
	}
}

// RemoveAllPods represents a phase.
type RemoveAllPods struct {
	Preserve bool
}

// Tasks implments the phase interface.
func (*RemoveAllPods) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.RemoveAllPods{},
	}
}

// ResetSystemDisk represents a phase.
type ResetSystemDisk struct {
	Preserve bool
}

// Tasks implments the phase interface.
func (*ResetSystemDisk) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.ResetSystemDisk{},
	}
}

// VerifySystemDiskAvailability represents a phase.
type VerifySystemDiskAvailability struct {
	Preserve bool
}

// Tasks implments the phase interface.
func (*VerifySystemDiskAvailability) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.VerifyDiskAvailability{},
	}
}

// Upgrade represents a phase.
type Upgrade struct {
	Preserve bool
}

// Tasks implments the phase interface.
func (*Upgrade) Tasks() []runtime.Task {
	return []runtime.Task{
		&task.Upgrade{},
	}
}
