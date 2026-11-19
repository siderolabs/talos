// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// ARM64 is the AArch64 platform: no legacy BIOS, no x86 platform timers, and libvirt's own video
// and input defaults.
type ARM64 struct{}

// QEMUArch implements [Platform].
func (ARM64) QEMUArch() string {
	return constants.KernelArchArm64
}

// RequiresUEFI implements [Platform].
func (ARM64) RequiresUEFI() bool {
	return true
}

// Clock implements [Platform].
func (ARM64) Clock(hypervisor.VirtualMachineProfileTimersSpec) *libvirtxml.DomainClock {
	return &libvirtxml.DomainClock{Offset: "utc"}
}

// Video implements [Platform].
func (ARM64) Video(*libvirtxml.Domain, hypervisor.VirtualMachineVideoSpec) {}

// DiskController implements [Platform].
func (ARM64) DiskController(domain *libvirtxml.Domain, bus string) {
	if bus == hypervisorhelpers.VirtualMachineDiskBusSCSI.String() {
		ensureVirtioSCSIController(domain)
	}
}

// SeedBus implements [Platform].
func (ARM64) SeedBus(domain *libvirtxml.Domain) string {
	ensureVirtioSCSIController(domain)

	return hypervisorhelpers.VirtualMachineDiskBusSCSI.String()
}

// Explicitly choose virtio-scsi rather than relying on libvirt's machine-specific
// implicit SCSI controller. The arm64 UEFI firmware has a virtio-scsi driver.
func ensureVirtioSCSIController(domain *libvirtxml.Domain) {
	for _, controller := range domain.Devices.Controllers {
		if controller.Type == "scsi" {
			return
		}
	}

	domain.Devices.Controllers = append(domain.Devices.Controllers, libvirtxml.DomainController{
		Type:  "scsi",
		Model: "virtio-scsi",
	})
}
