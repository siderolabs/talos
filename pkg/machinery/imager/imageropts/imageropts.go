// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package imageropts contains exportable types used in imager profile options.
package imageropts

//go:generate go tool github.com/dmarkham/enumer -type BootloaderKind -linecomment -text

// BootloaderKind is a bootloader for the disk image.
type BootloaderKind int

const (
	// BootLoaderKindNone is the zero value.
	BootLoaderKindNone BootloaderKind = iota // none
	// BootLoaderKindDualBoot is the dual-boot bootloader.
	// using sd-boot for UEFI and GRUB for BIOS.
	BootLoaderKindDualBoot // dual-boot
	// BootLoaderKindSDBoot is the sd-boot bootloader.
	BootLoaderKindSDBoot // sd-boot
	// BootLoaderKindGrub is the GRUB bootloader.
	BootLoaderKindGrub // grub
)
