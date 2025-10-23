// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package profile contains definition of the image generation profile.
package profile

import (
	"errors"
	"fmt"
	"io"

	"github.com/siderolabs/go-pointer"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/overlay"
)

//go:generate go tool github.com/siderolabs/deep-copy -type Profile -type SecureBootAssets -header-file ../../../hack/boilerplate.txt -o deep_copy.generated.go .

//go:generate go tool github.com/dmarkham/enumer -type OutputKind,OutFormat,DiskFormat,SDBootEnrollKeys,BootloaderKind -linecomment -text

// Profile describes image generation result.
type Profile struct {
	// BaseProfileName is the profile name to inherit from.
	BaseProfileName string `yaml:"baseProfileName,omitempty"`
	// Architecture of the image: amd64 or arm64.
	Arch string `yaml:"arch"`
	// Platform name of the image: qemu, aws, gcp, etc.
	Platform string `yaml:"platform"`
	// Board name of the image: rpi4, etc. (only for metal image and arm64).
	Board string `yaml:"board,omitempty"`
	// SecureBoot enables SecureBoot (only for UEFI build).
	SecureBoot *bool `yaml:"secureboot"`
	// Version is Talos version.
	Version string `yaml:"version"`
	// Various customizations than can be applied to the image.
	Customization CustomizationProfile `yaml:"customization,omitempty"`

	// Input describes inputs for image generation.
	Input Input `yaml:"input"`
	// Overlay describes overlay options for image generation.
	Overlay *OverlayOptions `yaml:"overlay,omitempty"`
	// Output describes image generation result.
	Output Output `yaml:"output"`
}

// OverlayOptions describes overlay options for image generation.
type OverlayOptions struct {
	// Name of the overlay installer, defaults to `default` if not set.
	Name string `yaml:"name"`
	// Image to use for the overlay.
	Image ContainerAsset `yaml:"image"`
	// Options for the overlay.
	overlay.ExtraOptions `yaml:"options,omitempty"` //nolint:embeddedstructfieldcheck
}

// SecureBootEnabled derefences SecureBoot.
func (p *Profile) SecureBootEnabled() bool {
	return pointer.SafeDeref(p.SecureBoot)
}

// Validate the profile.
//
//nolint:gocyclo,cyclop
func (p *Profile) Validate() error {
	if p.Arch != amd64 && p.Arch != arm64 {
		return fmt.Errorf("invalid arch %q", p.Arch)
	}

	if p.Platform == "" {
		return errors.New("platform is required")
	}

	if p.Board != "" {
		if p.Overlay != nil {
			return errors.New("overlay is not supported with board options")
		}

		if p.Arch != arm64 || p.Platform != "metal" {
			return errors.New("board is only supported for metal arm64")
		}
	}

	if p.SecureBootEnabled() && !quirks.New(p.Version).SupportsUKI() {
		return fmt.Errorf("secureboot is not supported for Talos version %q", p.Version)
	}

	switch p.Output.Kind {
	case OutKindUnknown:
		return errors.New("unknown output kind")
	case OutKindISO:
		// ISO supports all kinds of customization
		if quirks.New(p.Version).ISOSupportsSettingBootloader() {
			if p.Output.ISOOptions != nil && p.Output.ISOOptions.Bootloader == BootLoaderKindNone {
				return errors.New("bootloader cannot be 'none' for ISO output")
			}
		}
	case OutKindCmdline:
		// cmdline supports all kinds of customization
	case OutKindImage:
		// Image supports all kinds of customization
		if p.Output.ImageOptions == nil {
			return errors.New("image options are required for image output")
		}

		if p.Output.ImageOptions.DiskSize == 0 {
			return errors.New("disk size is required for image output")
		}

		if p.Output.ImageOptions.Bootloader == BootLoaderKindNone {
			return errors.New("bootloader cannot be 'none' for disk image output")
		}
	case OutKindInstaller:
		if len(p.Customization.MetaContents) > 0 {
			return fmt.Errorf("customization of meta partition is not supported for %s output", p.Output.Kind)
		}
	case OutKindKernel, OutKindInitramfs:
		if p.SecureBootEnabled() {
			return fmt.Errorf("secureboot is not supported for %s output", p.Output.Kind)
		}

		if len(p.Customization.ExtraKernelArgs) > 0 {
			return fmt.Errorf("customization of kernel args is not supported for %s output", p.Output.Kind)
		}

		if len(p.Customization.MetaContents) > 0 {
			return fmt.Errorf("customization of meta partition is not supported for %s output", p.Output.Kind)
		}
	case OutKindUKI:
	}

	return nil
}

// OutputPath generates the output path for the profile.
//
//nolint:gocyclo
func (p *Profile) OutputPath() string {
	path := p.Platform

	if p.Board != "" {
		path += "-" + p.Board
	}

	path += "-" + p.Arch

	if p.SecureBootEnabled() {
		path += "-secureboot"
	}

	switch p.Output.Kind {
	case OutKindUnknown:
		panic("unknown output kind")
	case OutKindISO:
		path += ".iso"
	case OutKindImage:
		path += "." + p.Output.ImageOptions.DiskFormat.String()
	case OutKindInstaller:
		path = "installer-" + p.Arch

		if p.SecureBootEnabled() {
			path += "-secureboot"
		}

		path += ".tar"
	case OutKindKernel:
		path = "kernel-" + p.Arch
	case OutKindInitramfs:
		path = "initramfs-" + path + ".xz"
	case OutKindUKI:
		path += "-uki.efi"
	case OutKindCmdline:
		path = "cmdline-" + path
	}

	return path
}

// Dump the profile as YAML.
func (p *Profile) Dump(w io.Writer) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)

	return encoder.Encode(p)
}
