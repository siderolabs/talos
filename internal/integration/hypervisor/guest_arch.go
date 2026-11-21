// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// builtinProfile is the hardware a builtin profile renders on the node's architecture.
type builtinProfile struct {
	name     string
	machine  string
	mode     hypervisorhelpers.VirtualMachineProfileCPUMode
	model    string
	fallback string
}

// guestArch collects what guests see differently on each node architecture.
type guestArch struct {
	// qemu is the libvirt guest architecture, and what uname -m reports in the guest.
	qemu string
	// firmware is the cheapest firmware stock guests boot with: BIOS where it exists.
	firmware hypervisorhelpers.VirtualMachineFirmwareType
	// serialTTY is the guest device behind the serial console.
	serialTTY string
	// cpuFeaturesField is the /proc/cpuinfo field listing CPU features, and cpuFeatures
	// a baseline every profile must expose.
	cpuFeaturesField string
	cpuFeatures      []string
	// profiles are the builtin profiles, "host" first.
	profiles []builtinProfile
	// timers is whether guests get the x86 platform timers (RTC, PIT, HPET).
	timers bool
}

var guestArches = map[string]guestArch{
	constants.ArchAmd64: {
		qemu:             constants.KernelArchAmd64,
		firmware:         hypervisorhelpers.VirtualMachineFirmwareTypeBIOS,
		serialTTY:        "ttyS0",
		cpuFeaturesField: "flags",
		cpuFeatures:      []string{"fpu", "lm"},
		profiles: []builtinProfile{
			{name: "host", machine: "q35", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel},
			{name: "passthrough", machine: "q35", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough},
			{
				name: "legacy", machine: "pc-i440fx-9.2", mode: hypervisorhelpers.VirtualMachineProfileCPUModeCustom,
				model: "kvm64", fallback: "forbid",
			},
		},
		timers: true,
	},
	constants.ArchArm64: {
		qemu:             constants.KernelArchArm64,
		firmware:         hypervisorhelpers.VirtualMachineFirmwareTypeUEFI,
		serialTTY:        "ttyAMA0",
		cpuFeaturesField: "Features",
		cpuFeatures:      []string{"fp", "asimd"},
		profiles: []builtinProfile{
			{name: "host", machine: "virt", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough},
			{name: "passthrough", machine: "virt", mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough},
		},
	},
}

// guestArch returns the guest expectations for the node's architecture.
func (suite *LibvirtSuite) guestArch(node string) guestArch {
	suite.T().Helper()

	nodeArch := suite.ReadMachineArch(client.WithNode(suite.ctx, node))

	arch, ok := guestArches[nodeArch]
	if !ok {
		suite.Require().FailNow("unsupported architecture", "architecture %q has no guest expectations", nodeArch)
	}

	return arch
}

// profile returns the named builtin profile.
func (arch guestArch) profile(name string) builtinProfile {
	for _, profile := range arch.profiles {
		if profile.name == name {
			return profile
		}
	}

	panic("unknown builtin profile " + name)
}

// defaultClock is the clock a guest gets from a profile without timer settings.
func (arch guestArch) defaultClock() *libvirtxml.DomainClock {
	if !arch.timers {
		return &libvirtxml.DomainClock{Offset: "utc"}
	}

	return defaultAlpineClock()
}
