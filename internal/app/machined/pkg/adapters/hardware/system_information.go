// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/siderolabs/go-smbios/smbios"

	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// SystemInformation adapter provider conversion from smbios.SMBIOS.
//
//nolint:revive,golint
func SystemInformation(p *hardware.SystemInformation) systemInformation {
	return systemInformation{
		SystemInformation: p,
	}
}

type systemInformation struct {
	*hardware.SystemInformation
}

// Update current systemInformation info.
func (p systemInformation) Update(systemInformation *smbios.SystemInformation, uuidRewrite string) {
	if uuidRewrite == "" {
		uuidRewrite = systemInformation.UUID
	}

	*p.SystemInformation.TypedSpec() = hardware.SystemInformationSpec{
		Manufacturer: systemInformation.Manufacturer,
		ProductName:  systemInformation.ProductName,
		Version:      systemInformation.Version,
		SerialNumber: systemInformation.SerialNumber,
		UUID:         uuidRewrite,
		WakeUpType:   systemInformation.WakeUpType.String(),
		SKUNumber:    systemInformation.SKUNumber,
	}
}
