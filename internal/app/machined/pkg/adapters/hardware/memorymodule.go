// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/siderolabs/go-smbios/smbios"

	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// MemoryModule adapter provider conversion from smbios.SMBIOS.
//
//nolint:revive,golint
func MemoryModule(m *hardware.MemoryModule) memoryModule {
	return memoryModule{
		MemoryModule: m,
	}
}

type memoryModule struct {
	*hardware.MemoryModule
}

// Update current processor info.
func (m memoryModule) Update(memory *smbios.MemoryDevice) {
	translateMemoryModuleInfo := func(in *smbios.MemoryDevice) hardware.MemoryModuleSpec {
		var memoryModuleSpec hardware.MemoryModuleSpec

		if in.Size != 0 && in.Size != 0xFFFF {
			var size uint32

			if in.Size == 0x7FFF {
				size = uint32(in.ExtendedSize)
			} else {
				size = uint32(in.Size)
			}

			memoryModuleSpec.AssetTag = in.AssetTag
			memoryModuleSpec.BankLocator = in.BankLocator
			memoryModuleSpec.DeviceLocator = in.DeviceLocator
			memoryModuleSpec.Manufacturer = in.Manufacturer
			memoryModuleSpec.ProductName = in.PartNumber
			memoryModuleSpec.SerialNumber = in.SerialNumber
			memoryModuleSpec.Size = size
			memoryModuleSpec.Speed = uint32(in.Speed)
		}

		return memoryModuleSpec
	}

	*m.MemoryModule.TypedSpec() = translateMemoryModuleInfo(memory)
}
