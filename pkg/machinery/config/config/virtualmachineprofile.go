// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"

// VirtualMachineProfileConfig describes guest hardware independently of individual VM sizing.
type VirtualMachineProfileConfig interface {
	NamedDocument
	VirtualMachineProfileConfigSignal()
	Machine() string
	CPU() VirtualMachineProfileCPUConfig
}

// VirtualMachineProfileCPUConfig selects the CPU exposed to a guest.
type VirtualMachineProfileCPUConfig interface {
	Mode() hypervisorhelpers.VirtualMachineProfileCPUMode
	Model() string
	Fallback() string
	Features() []VirtualMachineProfileCPUFeatureConfig
}

// VirtualMachineProfileCPUFeatureConfig configures one named CPUID feature.
type VirtualMachineProfileCPUFeatureConfig interface {
	Name() string
	Policy() hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicy
}
