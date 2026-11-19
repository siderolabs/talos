// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"fmt"

	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// Platform is the hardware a domain renders differently on each host architecture.
//
// Platforms are selected at runtime rather than by build constraints, so every architecture's
// rendering is tested on any build host.
type Platform interface {
	// QEMUArch is the libvirt guest architecture.
	QEMUArch() string
	// RequiresUEFI reports whether the architecture has no legacy BIOS.
	RequiresUEFI() bool
	// Clock renders the guest clock and platform timers.
	Clock(timers hypervisor.VirtualMachineProfileTimersSpec) *libvirtxml.DomainClock
	// Video adds the display devices of a VNC console.
	Video(domain *libvirtxml.Domain, video hypervisor.VirtualMachineVideoSpec)
	// DiskController adds any controller a disk on the bus needs.
	DiskController(domain *libvirtxml.Domain, bus string)
	// SeedBus prepares and returns the bus of the cloud-init seed CD-ROM.
	SeedBus(domain *libvirtxml.Domain) string
}

// PlatformFor returns the platform of a host architecture, given as GOARCH.
func PlatformFor(arch string) (Platform, error) {
	switch arch {
	case constants.ArchAmd64:
		return AMD64{}, nil
	case constants.ArchArm64:
		return ARM64{}, nil
	default:
		return nil, fmt.Errorf("architecture %q is unsupported", arch)
	}
}
