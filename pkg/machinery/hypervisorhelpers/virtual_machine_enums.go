// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

// VirtualMachineProfileCPUMode selects the guest CPU exposure mode.
type VirtualMachineProfileCPUMode int

// Profile CPU modes.
//
//structprotogen:gen_enum
const (
	VirtualMachineProfileCPUModeUnknown         VirtualMachineProfileCPUMode = iota // unknown
	VirtualMachineProfileCPUModeHostModel                                           // host-model
	VirtualMachineProfileCPUModeHostPassthrough                                     // host-passthrough
	VirtualMachineProfileCPUModeCustom                                              // custom
)

// VirtualMachineProfileCPUFeaturePolicy selects a libvirt CPU feature policy.
type VirtualMachineProfileCPUFeaturePolicy int

// Profile CPU feature policies.
//
//structprotogen:gen_enum
const (
	VirtualMachineProfileCPUFeaturePolicyUnknown  VirtualMachineProfileCPUFeaturePolicy = iota // unknown
	VirtualMachineProfileCPUFeaturePolicyRequire                                               // require
	VirtualMachineProfileCPUFeaturePolicyDisable                                               // disable
	VirtualMachineProfileCPUFeaturePolicyOptional                                              // optional
	VirtualMachineProfileCPUFeaturePolicyForce                                                 // force
	VirtualMachineProfileCPUFeaturePolicyForbid                                                // forbid
)

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
