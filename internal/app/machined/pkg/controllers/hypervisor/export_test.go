// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import "github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"

var (
	ValidateDomainPlacement = validateDomainPlacement
	IsPlacementHeld         = isPlacementHeld
	ErrPlacementPending     = errPlacementPending
	ErrPlacementInvalid     = errPlacementInvalid

	CheckDomainHostDevices           = checkDomainHostDevices
	ErrHostDevicePending             = errHostDevicePending
	ErrHostDeviceUnavailable         = errHostDeviceUnavailable
	ErrVFIOBackendPending            = errVFIOBackendPending
	ErrHostDeviceClaimed             = errHostDeviceClaimed
	CheckPCIClaims                   = checkPCIClaims
	ValidateVirtualMachinePCIDevices = validateVirtualMachinePCIDevices
)

// BlankVolumeNameForTest exposes the volume naming rule, which is the one thing keeping two virtual
// machines out of each other's disks.
func BlankVolumeNameForTest(virtualMachine string, disk hypervisor.VirtualMachineDiskSpec) string {
	name, err := blankVolumeName(virtualMachine, disk)
	if err != nil {
		panic(err)
	}

	return name
}

// PCIClaim is the PCI devices another virtual machine passes through.
type PCIClaim = pciClaim

// NewPCIClaimForTest builds the claim of another virtual machine on the given devices.
func NewPCIClaimForTest(name string, running bool, addresses ...string) PCIClaim {
	return pciClaim{name: name, running: running, addresses: addresses}
}
