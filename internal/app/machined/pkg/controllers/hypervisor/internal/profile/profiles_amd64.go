// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build amd64

package profile

import (
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// Profiles returns the stable builtin catalog in name order.
// Each call returns fresh resources so callers cannot modify the catalog.
func Profiles() []*hypervisor.VirtualMachineProfileSpec {
	host := hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, "host")
	*host.TypedSpec() = hypervisor.VirtualMachineProfileSpecSpec{
		Builtin: true,
		Machine: "q35",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
		},
	}

	// Legacy exposes a deliberately limited CPU baseline and traditional PC chipset.
	// Firmware and guest device drivers remain separate VM settings.
	legacy := hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, "legacy")
	*legacy.TypedSpec() = hypervisor.VirtualMachineProfileSpecSpec{
		Builtin: true,
		Machine: "pc-i440fx-9.2",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode:     hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
			Model:    "kvm64",
			Fallback: "forbid",
		},
	}

	passthrough := hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, "passthrough")
	*passthrough.TypedSpec() = hypervisor.VirtualMachineProfileSpecSpec{
		Builtin: true,
		Machine: "q35",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
		},
	}

	return []*hypervisor.VirtualMachineProfileSpec{
		host,
		legacy,
		passthrough,
	}
}

func validMachine(machine string) bool {
	return hypervisorhelpers.ValidVirtualMachineProfileMachine(machine)
}
