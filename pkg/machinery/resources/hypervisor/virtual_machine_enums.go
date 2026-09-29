// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

// VirtualMachinePowerState is the observed power state of a virtual machine.
type VirtualMachinePowerState int

// Virtual machine power states.
//
//structprotogen:gen_enum
const (
	VirtualMachinePowerStateUnknown VirtualMachinePowerState = iota // unknown
	VirtualMachinePowerStateRunning                                 // running
	VirtualMachinePowerStateStopped                                 // stopped
)

// VirtualMachineStage describes how far the observed VM has converged to its desired state.
type VirtualMachineStage int

// Virtual machine reconciliation stages.
//
//structprotogen:gen_enum
const (
	VirtualMachineStageUnknown VirtualMachineStage = iota // unknown
	VirtualMachineStagePending                            // pending
	VirtualMachineStageReady                              // ready
	VirtualMachineStageError                              // error
)
