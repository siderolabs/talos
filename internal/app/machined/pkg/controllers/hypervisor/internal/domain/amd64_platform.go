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

// AMD64 is the x86-64 platform.
type AMD64 struct{}

// QEMUArch implements [Platform].
func (AMD64) QEMUArch() string {
	return constants.KernelArchAmd64
}

// RequiresUEFI implements [Platform].
func (AMD64) RequiresUEFI() bool {
	return false
}

// Clock implements [Platform].
func (AMD64) Clock(timers hypervisor.VirtualMachineProfileTimersSpec) *libvirtxml.DomainClock {
	return &libvirtxml.DomainClock{
		Offset: "utc",
		Timer: []libvirtxml.DomainTimer{
			{Name: "rtc", TickPolicy: timers.RTCTickPolicy.String()},
			{Name: "pit", TickPolicy: timers.PITTickPolicy.String()},
			{Name: "hpet", Present: libvirtYesNo(timers.HPET)},
		},
	}
}

// Video implements [Platform].
func (AMD64) Video(domain *libvirtxml.Domain, video hypervisor.VirtualMachineVideoSpec) {
	// Default to VGA; an explicitly selected model may require guest drivers.
	videoModel := video.Model
	if videoModel == "" {
		videoModel = "vga"
	}

	videoDev := libvirtxml.DomainVideo{Model: libvirtxml.DomainVideoModel{Type: videoModel}}
	if video.VRAMMiB > 0 {
		// libvirt's vram attribute is in KiB.
		videoDev.Model.VRam = uint(video.VRAMMiB) * 1024
	}

	domain.Devices.Videos = []libvirtxml.DomainVideo{videoDev}
	domain.Devices.Controllers = append(domain.Devices.Controllers, libvirtxml.DomainController{
		Type: "usb", Model: "qemu-xhci", USB: &libvirtxml.DomainControllerUSB{},
	})
	domain.Devices.Inputs = []libvirtxml.DomainInput{
		{Type: "tablet", Bus: "usb"},
		{Type: "keyboard", Bus: "usb"},
	}
}

// DiskController implements [Platform]: libvirt adds any controller the disk needs.
func (AMD64) DiskController(*libvirtxml.Domain, string) {}

// SeedBus implements [Platform].
func (AMD64) SeedBus(*libvirtxml.Domain) string {
	return hypervisorhelpers.VirtualMachineDiskBusSATA.String()
}
