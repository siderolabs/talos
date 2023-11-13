// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package profile contains definition of the image generation profile.
package profile

import (
	"fmt"
	"io"

	"github.com/siderolabs/go-pointer"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/meta"
)

//go:generate deep-copy -type Profile -header-file ../../../hack/boilerplate.txt -o deep_copy.generated.go .

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
	// Output describes image generation result.
	Output Output `yaml:"output"`
}

// CustomizationProfile describes customizations that can be applied to the image.
type CustomizationProfile struct {
	// ExtraKernelArgs is a list of extra kernel arguments.
	ExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	// MetaContents is a list of META partition contents.
	MetaContents meta.Values `yaml:"metaContents,omitempty"`
}

// SecureBootEnabled derefences SecureBoot.
func (p *Profile) SecureBootEnabled() bool {
	return pointer.SafeDeref(p.SecureBoot)
}

// Validate the profile.
//
//nolint:gocyclo
func (p *Profile) Validate() error {
	if p.Arch != "amd64" && p.Arch != "arm64" {
		return fmt.Errorf("invalid arch %q", p.Arch)
	}

	if p.Platform == "" {
		return fmt.Errorf("platform is required")
	}

	if p.Board != "" {
		if !(p.Arch == "arm64" && p.Platform == "metal") {
			return fmt.Errorf("board is only supported for metal arm64")
		}
	}

	switch p.Output.Kind {
	case OutKindUnknown:
		return fmt.Errorf("unknown output kind")
	case OutKindISO:
		// ISO supports all kinds of customization
	case OutKindImage:
		// Image supports all kinds of customization
		if p.Output.ImageOptions.DiskSize == 0 {
			return fmt.Errorf("disk size is required for image output")
		}
	case OutKindInstaller:
		if !p.SecureBootEnabled() && len(p.Customization.ExtraKernelArgs) > 0 {
			return fmt.Errorf("customization of kernel args is not supported for %s output in !secureboot mode", p.Output.Kind)
		}

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
		if !p.SecureBootEnabled() {
			return fmt.Errorf("!secureboot is not supported for %s output", p.Output.Kind)
		}
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
		path += "-installer.tar"
	case OutKindKernel:
		path = "kernel-" + p.Arch
	case OutKindInitramfs:
		path = "initramfs-" + path + ".xz"
	case OutKindUKI:
		path += "-uki.efi"
	}

	return path
}

// Dump the profile as YAML.
func (p *Profile) Dump(w io.Writer) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)

	return encoder.Encode(p)
}
