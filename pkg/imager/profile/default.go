// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	mib = 1024 * 1024

	// MinRAWDiskSize is the minimum size disk we can create. Used for metal images.
	MinRAWDiskSize = 1246 * mib

	// DefaultRAWDiskSize is the value we use for any non-metal images by default.
	DefaultRAWDiskSize = 8192 * mib
)

// Default describes built-in profiles.
var Default = map[string]Profile{
	// ISO
	"iso": {
		Platform:   constants.PlatformMetal,
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindISO,
			OutFormat: OutFormatRaw,
		},
	},
	"secureboot-iso": {
		Platform:   constants.PlatformMetal,
		SecureBoot: new(true),
		Output: Output{
			Kind:      OutKindISO,
			OutFormat: OutFormatRaw,
			ISOOptions: &ISOOptions{
				SDBootEnrollKeys: SDBootEnrollKeysIfSafe,
			},
		},
	},
	// Metal images
	"metal": {
		Platform:   constants.PlatformMetal,
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"metal-uki": {
		Platform:   constants.PlatformMetal,
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindUKI,
			OutFormat: OutFormatRaw,
		},
	},
	"secureboot-metal-uki": {
		Platform:   constants.PlatformMetal,
		SecureBoot: new(true),
		Output: Output{
			Kind:      OutKindUKI,
			OutFormat: OutFormatRaw,
		},
	},
	"secureboot-metal": {
		Platform:   constants.PlatformMetal,
		SecureBoot: new(true),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"installer": {
		Platform:   "metal",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindInstaller,
			OutFormat: OutFormatRaw,
		},
	},
	"secureboot-installer": {
		Platform:   "metal",
		SecureBoot: new(true),
		Output: Output{
			Kind:      OutKindInstaller,
			OutFormat: OutFormatRaw,
		},
	},
	// Clouds
	"akamai": {
		Platform:   "akamai",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatGZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"aws": {
		Platform:   "aws",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"azure": {
		Platform:   "azure",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:          DefaultRAWDiskSize,
				DiskFormat:        DiskFormatVPC,
				DiskFormatOptions: "subformat=fixed,force_size",
			},
		},
	},
	"cloudstack": {
		Platform:   "cloudstack",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"digital-ocean": {
		Platform:   "digital-ocean",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatGZ,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"exoscale": {
		Platform:   "exoscale",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:          10 * 1024 * mib,
				DiskFormat:        DiskFormatQCOW2,
				DiskFormatOptions: "cluster_size=8k",
			},
		},
	},
	"gcp": {
		Platform:   "gcp",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatTar,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"hcloud": {
		Platform:   "hcloud",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"nocloud": {
		Platform:   "nocloud",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"opennebula": {
		Platform:   "opennebula",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"openstack": {
		Platform:   "openstack",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"oracle": {
		Platform:   "oracle",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:          DefaultRAWDiskSize,
				DiskFormat:        DiskFormatQCOW2,
				DiskFormatOptions: "cluster_size=8k",
			},
		},
	},
	"scaleway": {
		Platform:   "scaleway",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"upcloud": {
		Platform:   "upcloud",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"vmware": {
		Platform:   "vmware",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatRaw,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatOVA,
			},
		},
	},
	"vultr": {
		Platform:   "vultr",
		SecureBoot: new(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatZSTD,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
}
