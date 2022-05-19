// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/talos-systems/go-smbios/smbios"

	"github.com/talos-systems/talos/pkg/machinery/resources/hardware"
)

// Memory adapter provider conversion from smbios.SMBIOS.
//
//nolint:revive,golint
func Memory(m *hardware.MemoryInfo) memory {
	return memory{
		MemoryInfo: m,
	}
}

type memory struct {
	*hardware.MemoryInfo
}

// Update current processor info.
func (m memory) Update(memory *smbios.MemoryDevice) {
	translateProcessorInfo := func(in *smbios.MemoryDevice) hardware.MemorySpec {
		var memorySpec hardware.MemorySpec

		if in.Size != 0 && in.Size != 0xFFFF {
			var size uint32

			if in.Size == 0x7FFF {
				size = uint32(in.ExtendedSize)
			} else {
				size = uint32(in.Size)
			}

			memorySpec.AssetTag = in.AssetTag
			memorySpec.BankLocator = in.BankLocator
			memorySpec.DeviceLocator = in.DeviceLocator
			memorySpec.Manufacturer = in.Manufacturer
			memorySpec.ProductName = in.PartNumber
			memorySpec.SerialNumber = in.SerialNumber
			memorySpec.Size = size
			memorySpec.Speed = uint32(in.Speed)
		}

		return memorySpec
	}

	*m.MemoryInfo.TypedSpec() = translateProcessorInfo(memory)
}
