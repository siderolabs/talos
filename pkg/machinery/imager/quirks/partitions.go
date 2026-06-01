// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package quirks

// PartitionSizes capture partition sizes for Talos.
type PartitionSizes struct {
	bootSize uint64
}

const (
	mib = 1024 * 1024
	gib = 1024 * mib
)

// GrubEFISize return EFI partition size for GRUB layout.
func (p PartitionSizes) GrubEFISize() uint64 {
	return 100 * mib
}

// GrubBIOSSize return BIOS GRUB partition size.
func (p PartitionSizes) GrubBIOSSize() uint64 {
	return 1 * mib
}

// GrubBootSize return boot partition size for GRUB layout.
func (p PartitionSizes) GrubBootSize() uint64 {
	return p.bootSize
}

// UKIEFISize return EFI partition size for UKI layout.
func (p PartitionSizes) UKIEFISize() uint64 {
	// EFIUKISize is the size of the EFI partition when UKI is enabled.
	// With UKI all assets are stored in the EFI partition.
	// This is the size of the old EFISize + BIOSGrubSize + BootSize.
	return p.GrubEFISize() + p.GrubBIOSSize() + p.GrubBootSize()
}

// METASize return META partition size.
func (p PartitionSizes) METASize() uint64 {
	return 1 * mib
}

// StateSize return state partition size.
func (p PartitionSizes) StateSize() uint64 {
	return 100 * mib
}

// EphemeralMinSize return minimum size for ephemeral partition.
func (p PartitionSizes) EphemeralMinSize() uint64 {
	return 2 * gib
}
