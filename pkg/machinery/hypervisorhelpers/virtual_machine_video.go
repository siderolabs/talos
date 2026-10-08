// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import "fmt"

// ValidateVirtualMachineVideo checks the models supported by the QEMU extension
// and rejects VRAM values that the backend would ignore, clamp, or round.
func ValidateVirtualMachineVideo(model string, vramMiB uint32) error {
	var minVRAM, maxVRAM uint32

	switch model {
	case "", "vga":
		// QEMU vga_common_init clamps to 1..512 MiB, then rounds up to a power of two.
		minVRAM, maxVRAM = 1, 512
	case "bochs":
		// QEMU bochs_display_realize requires 4..256 MiB, then rounds up to a power of two.
		minVRAM, maxVRAM = 4, 256
	case "virtio", "cirrus":
		// libvirt does not forward vram to these QEMU devices.
		if vramMiB != 0 {
			return fmt.Errorf("video.vramMiB is not supported for model %q", model)
		}

		return nil
	default:
		// In particular, QXL requires SPICE, which the QEMU extension disables.
		return fmt.Errorf("unsupported video.model %q (expected vga, virtio, bochs, or cirrus)", model)
	}

	if vramMiB != 0 && (vramMiB < minVRAM || vramMiB > maxVRAM || vramMiB&(vramMiB-1) != 0) {
		return fmt.Errorf("video.vramMiB must be zero or a power of two between %d and %d MiB for model %q", minVRAM, maxVRAM, model)
	}

	return nil
}
