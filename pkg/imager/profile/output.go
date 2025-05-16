// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Output describes image generation result.
type Output struct {
	// Kind of the output:
	//  * iso - ISO image
	//  * image - disk image (Talos pre-installed)
	//  * installer - installer container
	//  * kernel - Linux kernel
	//  * initramfs - initramfs image
	Kind OutputKind `yaml:"kind"`
	// Options for the 'image' output.
	ImageOptions *ImageOptions `yaml:"imageOptions,omitempty"`
	// Options for the 'iso' output.
	ISOOptions *ISOOptions `yaml:"isoOptions,omitempty"`
	// OutFormat is the format for the output:
	//  * raw - output raw file
	//  * .tar.gz - output tar.gz archive
	//  * .xz - output xz archive
	//  * .gz - output gz archive
	OutFormat OutFormat `yaml:"outFormat"`
}

// ImageOptions describes options for the 'image' output.
type ImageOptions struct {
	// DiskSize is the size of the disk image (bytes).
	DiskSize int64 `yaml:"diskSize"`
	// DiskFormat is the format of the disk image:
	//  * raw - raw disk image
	//  * qcow2 - qcow2 disk image
	//  * vhd - VPC disk image
	//  * ova - VMWare disk image
	DiskFormat DiskFormat `yaml:"diskFormat,omitempty"`
	// DiskFormatOptions are additional options for the disk format
	DiskFormatOptions string `yaml:"diskFormatOptions,omitempty"`
	// Bootloader is the bootloader to use for the disk image.
	// If not set, it defaults to dual-boot.
	Bootloader DiskImageBootloader `yaml:"bootloader"`
}

// ISOOptions describes options for the 'iso' output.
type ISOOptions struct {
	// SDBootEnrollKeys is a value in loader.conf secure-boot-enroll: off, manual, if-safe, force.
	//
	// If not set, it defaults to if-safe.
	SDBootEnrollKeys SDBootEnrollKeys `yaml:"sdBootEnrollKeys"`
}

// OutputKind is output specification.
type OutputKind int

// OutputKind values.
const (
	OutKindUnknown   OutputKind = iota // unknown
	OutKindISO                         // iso
	OutKindImage                       // image
	OutKindInstaller                   // installer
	OutKindKernel                      // kernel
	OutKindInitramfs                   // initramfs
	OutKindUKI                         // uki
	OutKindCmdline                     // cmdline
)

// OutFormat is output format specification.
type OutFormat int

// OutFormat values.
const (
	OutFormatUnknown OutFormat = iota // unknown
	OutFormatRaw                      // raw
	OutFormatTar                      // .tar.gz
	OutFormatXZ                       // .xz
	OutFormatGZ                       // .gz
	OutFormatZSTD                     // .zst
)

// DiskFormat is disk format specification.
type DiskFormat int

// DiskFormat values.
const (
	DiskFormatUnknown DiskFormat = iota // unknown
	DiskFormatRaw                       // raw
	DiskFormatQCOW2                     // qcow2
	DiskFormatVPC                       // vhd
	DiskFormatOVA                       // ova
)

// SDBootEnrollKeys is a value in loader.conf secure-boot-enroll: off, manual, if-safe, force.
type SDBootEnrollKeys int

// SDBootEnrollKeys values.
const (
	SDBootEnrollKeysIfSafe SDBootEnrollKeys = iota // if-safe
	SDBootEnrollKeysManual                         // manual
	SDBootEnrollKeysForce                          // force
	SDBootEnrollKeysOff                            // off
)

// DiskImageBootloader is a bootloader for the disk image.
type DiskImageBootloader int

const (
	// DiskImageBootloaderDualBoot is the dual-boot bootloader
	// using sd-boot for UEFI and GRUB for BIOS.
	DiskImageBootloaderDualBoot DiskImageBootloader = iota // dual-boot
	// DiskImageBootloaderSDBoot is the sd-boot bootloader.
	DiskImageBootloaderSDBoot // sd-boot
	// DiskImageBootloaderGrub is the GRUB bootloader.
	DiskImageBootloaderGrub // grub
)

// FillDefaults fills default values for the output.
//
//nolint:gocyclo
func (o *Output) FillDefaults(arch, version string, secureboot bool) {
	if o.Kind == OutKindImage {
		if o.ImageOptions == nil {
			o.ImageOptions = &ImageOptions{}
		}

		// allow user to override bootloader
		if o.ImageOptions.Bootloader != DiskImageBootloaderDualBoot {
			return
		}

		useSDBoot := quirks.New(version).UseSDBootForUEFI()

		switch {
		case secureboot:
			o.ImageOptions.Bootloader = DiskImageBootloaderSDBoot
		case arch == "arm64" && useSDBoot:
			o.ImageOptions.Bootloader = DiskImageBootloaderSDBoot
		case !useSDBoot:
			o.ImageOptions.Bootloader = DiskImageBootloaderGrub
		default:
			o.ImageOptions.Bootloader = DiskImageBootloaderDualBoot
		}

		if o.ImageOptions.Bootloader == DiskImageBootloaderDualBoot {
			// add extra space for BIOS and BOOT partitions
			o.ImageOptions.DiskSize += 1*1024*1024 + 1000*1024*1024
		}

		if quirks.New(version).Uses2GBBoot() {
			// increase size for 2GB boot
			o.ImageOptions.DiskSize += 1000 * 1024 * 1024

			if o.ImageOptions.Bootloader == DiskImageBootloaderDualBoot {
				// bump once more, for dual boot
				o.ImageOptions.DiskSize += 1000 * 1024 * 1024
			}
		}
	}
}
