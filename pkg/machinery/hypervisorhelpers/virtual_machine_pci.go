// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// VFIOPCIDriver is the host driver a PCI device must be bound to before it can be passed through.
const VFIOPCIDriver = "vfio-pci"

// pciAddressPattern is the full sysfs form of a PCI address: domain, bus, slot and function.
var pciAddressPattern = regexp.MustCompile(`^([0-9a-f]{4}):([0-9a-f]{2}):([0-9a-f]{2})\.([0-7])$`)

// PCIAddress is a host PCI address split into its components.
type PCIAddress struct {
	Domain   uint16
	Bus      uint8
	Slot     uint8
	Function uint8
}

// String returns the address in its canonical sysfs form, e.g. "0000:41:00.0".
func (a PCIAddress) String() string {
	return fmt.Sprintf("%04x:%02x:%02x.%x", a.Domain, a.Bus, a.Slot, a.Function)
}

// ParsePCIAddress parses a host PCI address in its full sysfs form, e.g. "0000:41:00.0".
//
// The domain is required: a short "41:00.0" is ambiguous on hosts with more than one PCI segment.
// Hexadecimal digits are accepted in either case; String returns the lowercase form sysfs uses.
func ParsePCIAddress(address string) (PCIAddress, error) {
	match := pciAddressPattern.FindStringSubmatch(strings.ToLower(address))
	if match == nil {
		return PCIAddress{}, fmt.Errorf("PCI address %q must be in the form dddd:bb:ss.f, e.g. 0000:41:00.0", address)
	}

	// The pattern bounds every component, so the conversions below cannot fail or overflow.
	domain, _ := strconv.ParseUint(match[1], 16, 16) //nolint:errcheck
	bus, _ := strconv.ParseUint(match[2], 16, 8)     //nolint:errcheck
	slot, _ := strconv.ParseUint(match[3], 16, 8)    //nolint:errcheck
	function, _ := strconv.ParseUint(match[4], 8, 8) //nolint:errcheck

	// A PCI bus carries 32 slots; anything past that is not an address the kernel can produce.
	if slot > 0x1f {
		return PCIAddress{}, fmt.Errorf("PCI address %q: slot must be between 00 and 1f", address)
	}

	return PCIAddress{
		Domain:   uint16(domain),
		Bus:      uint8(bus),
		Slot:     uint8(slot),
		Function: uint8(function),
	}, nil
}
