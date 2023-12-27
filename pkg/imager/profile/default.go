// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile

import (
	"github.com/siderolabs/go-pointer"

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
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindISO,
			OutFormat: OutFormatRaw,
		},
	},
	"secureboot-iso": {
		Platform:   constants.PlatformMetal,
		SecureBoot: pointer.To(true),
		Output: Output{
			Kind:      OutKindISO,
			OutFormat: OutFormatRaw,
		},
	},
	// Metal images
	"metal": {
		Platform:   constants.PlatformMetal,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"secureboot-metal": {
		Platform:   constants.PlatformMetal,
		SecureBoot: pointer.To(true),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"installer": {
		Platform:   "metal",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindInstaller,
			OutFormat: OutFormatRaw,
		},
	},
	"secureboot-installer": {
		Platform:   "metal",
		SecureBoot: pointer.To(true),
		Output: Output{
			Kind:      OutKindInstaller,
			OutFormat: OutFormatRaw,
		},
	},
	// Clouds
	"aws": {
		Platform:   "aws",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"azure": {
		Platform:   "azure",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:          DefaultRAWDiskSize,
				DiskFormat:        DiskFormatVPC,
				DiskFormatOptions: "subformat=fixed,force_size",
			},
		},
	},
	"digital-ocean": {
		Platform:   "digital-ocean",
		SecureBoot: pointer.To(false),
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
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:          10 * 1024 * mib,
				DiskFormat:        DiskFormatQCOW2,
				DiskFormatOptions: "cluster_size=8k",
			},
		},
	},
	"gcp": {
		Platform:   "gcp",
		SecureBoot: pointer.To(false),
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
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"nocloud": {
		Platform:   "nocloud",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"openstack": {
		Platform:   "openstack",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"oracle": {
		Platform:   "oracle",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:          DefaultRAWDiskSize,
				DiskFormat:        DiskFormatQCOW2,
				DiskFormatOptions: "cluster_size=8k",
			},
		},
	},
	"scaleway": {
		Platform:   "scaleway",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"upcloud": {
		Platform:   "upcloud",
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	"vmware": {
		Platform:   "vmware",
		SecureBoot: pointer.To(false),
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
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   DefaultRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	// SBCs
	constants.BoardRPiGeneric: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardRPiGeneric,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardRock64: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardRock64,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardBananaPiM64: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardBananaPiM64,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardLibretechAllH3CCH5: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardLibretechAllH3CCH5,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardRockpi4: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardRockpi4,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardRockpi4c: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardRockpi4c,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardPine64: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardPine64,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardJetsonNano: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardJetsonNano,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardNanoPiR4S: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardNanoPiR4S,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
	constants.BoardSOQuartzCM4: {
		Arch:       "arm64",
		Platform:   constants.PlatformMetal,
		Board:      constants.BoardSOQuartzCM4,
		SecureBoot: pointer.To(false),
		Output: Output{
			Kind:      OutKindImage,
			OutFormat: OutFormatXZ,
			ImageOptions: &ImageOptions{
				DiskSize:   MinRAWDiskSize,
				DiskFormat: DiskFormatRaw,
			},
		},
	},
}
