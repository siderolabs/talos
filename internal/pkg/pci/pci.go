// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pci provides methods to access PCI-related data.
package pci

import (
	"github.com/siderolabs/go-pcidb/pkg/pcidb"
)

// Device describes PCI device.
type Device struct {
	VendorID  uint16
	ProductID uint16

	Vendor  string
	Product string
}

// LookupDB looks up device info in the PCI database.
func (d *Device) LookupDB() {
	d.Vendor, _ = pcidb.LookupVendor(d.VendorID)
	d.Product, _ = pcidb.LookupProduct(d.VendorID, d.ProductID)
}
