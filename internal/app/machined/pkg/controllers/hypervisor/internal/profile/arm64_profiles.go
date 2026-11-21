// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// arm64Profiles is the arm64 builtin catalog in name order.
//
// KVM on arm64 exposes only the host CPU: libvirt rejects host-model and named models
// need TCG, so both profiles pass the host CPU through. There is no legacy chipset, and no x86
// platform timers to set.
func arm64Profiles() []*hypervisor.VirtualMachineProfileSpec {
	return []*hypervisor.VirtualMachineProfileSpec{
		passthroughProfile("host"),
		passthroughProfile("passthrough"),
	}
}

func passthroughProfile(name string) *hypervisor.VirtualMachineProfileSpec {
	profile := hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, name)
	*profile.TypedSpec() = hypervisor.VirtualMachineProfileSpecSpec{
		Builtin: true,
		Machine: "virt",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
		},
	}

	return profile
}
