// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/siderolabs/go-smbios/smbios"

	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// Processor adapter provider conversion from smbios.SMBIOS.
//
//nolint:revive,golint
func Processor(p *hardware.Processor) processor {
	return processor{
		Processor: p,
	}
}

type processor struct {
	*hardware.Processor
}

// Update current processor info.
func (p processor) Update(processor *smbios.ProcessorInformation) {
	translateProcessorInfo := func(in *smbios.ProcessorInformation) hardware.ProcessorSpec {
		var processorSpec hardware.ProcessorSpec

		if in.Status.SocketPopulated() {
			processorSpec.Socket = in.SocketDesignation
			processorSpec.Manufacturer = in.ProcessorManufacturer
			processorSpec.ProductName = in.ProcessorVersion
			processorSpec.MaxSpeed = uint32(in.MaxSpeed)
			processorSpec.BootSpeed = uint32(in.CurrentSpeed)
			processorSpec.Status = uint32(in.Status)
			processorSpec.SerialNumber = in.SerialNumber
			processorSpec.AssetTag = in.AssetTag
			processorSpec.PartNumber = in.PartNumber
			processorSpec.CoreCount = uint32(in.CoreCount)
			processorSpec.CoreEnabled = uint32(in.CoreEnabled)
			processorSpec.ThreadCount = uint32(in.ThreadCount)
		}

		return processorSpec
	}

	*p.Processor.TypedSpec() = translateProcessorInfo(processor)
}
