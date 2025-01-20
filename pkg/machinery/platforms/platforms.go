// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package platforms provides meta information about supported Talos platforms, boards, etc.
package platforms

import (
	"fmt"

	"github.com/blang/semver/v4"
)

// Arch represents an architecture supported by Talos.
type Arch = string

// Architecture constants.
const (
	ArchAmd64 = "amd64"
	ArchArm64 = "arm64"
)

// BootMethod represents a boot method supported by Talos.
type BootMethod = string

// BootMethod constants.
const (
	BootMethodDiskImage = "disk-image"
	BootMethodISO       = "iso"
	BootMethodPXE       = "pxe"
)

// Platform represents a platform supported by Talos.
type Platform struct {
	Name string

	Label       string
	Description string

	MinVersion          semver.Version
	Architectures       []Arch
	Documentation       string
	DiskImageSuffix     string
	BootMethods         []BootMethod
	SecureBootSupported bool
}

// NotOnlyDiskImage is true if the platform supports boot methods other than disk-image.
func (p Platform) NotOnlyDiskImage() bool {
	if len(p.BootMethods) == 1 && p.BootMethods[0] == BootMethodDiskImage {
		return false
	}

	return true
}

// DiskImageDefaultPath returns the path to the disk image for the platform.
func (p Platform) DiskImageDefaultPath(arch Arch) string {
	return p.DiskImagePath(arch, p.DiskImageSuffix)
}

// SecureBootDiskImageDefaultPath returns the path to the SecureBoot disk image for the platform.
func (p Platform) SecureBootDiskImageDefaultPath(arch Arch) string {
	return p.SecureBootDiskImagePath(arch, p.DiskImageSuffix)
}

// DiskImagePath returns the path to the disk image for the platform and suffix.
func (p Platform) DiskImagePath(arch Arch, suffix string) string {
	return fmt.Sprintf("%s-%s.%s", p.Name, arch, suffix)
}

// SecureBootDiskImagePath returns the path to the SecureBoot disk image for the platform and suffix.
func (p Platform) SecureBootDiskImagePath(arch Arch, suffix string) string {
	return fmt.Sprintf("%s-%s-secureboot.%s", p.Name, arch, suffix)
}

// ISOPath returns the path to the ISO for the platform.
func (p Platform) ISOPath(arch Arch) string {
	return fmt.Sprintf("%s-%s.iso", p.Name, arch)
}

// SecureBootISOPath returns the path to the SecureBoot ISO for the platform.
func (p Platform) SecureBootISOPath(arch Arch) string {
	return fmt.Sprintf("%s-%s-secureboot.iso", p.Name, arch)
}

// PXEScriptPath returns the path to the PXE script for the platform.
func (p Platform) PXEScriptPath(arch Arch) string {
	return fmt.Sprintf("%s-%s", p.Name, arch)
}

// SecureBootPXEScriptPath returns the path to the SecureBoot PXE script for the platform.
func (p Platform) SecureBootPXEScriptPath(arch Arch) string {
	return fmt.Sprintf("%s-%s-secureboot", p.Name, arch)
}

// UKIPath returns the path to the UKI for the platform.
func (p Platform) UKIPath(arch Arch) string {
	return fmt.Sprintf("%s-%s-uki.efi", p.Name, arch)
}

// SecureBootUKIPath returns the path to the SecureBoot UKI for the platform.
func (p Platform) SecureBootUKIPath(arch Arch) string {
	return fmt.Sprintf("%s-%s-secureboot-uki.efi", p.Name, arch)
}

// KernelPath returns the path to the kernel for the platform.
//
// NB: Kernel path doesn't depend on the platform.
func (p Platform) KernelPath(arch Arch) string {
	return fmt.Sprintf("kernel-%s", arch)
}

// InitramfsPath returns the path to the initramfs for the platform.
//
// NB: Initramfs path doesn't depend on the platform.
func (p Platform) InitramfsPath(arch Arch) string {
	return fmt.Sprintf("initramfs-%s.xz", arch)
}

// CmdlinePath returns the path to the cmdline for the platform.
func (p Platform) CmdlinePath(arch Arch) string {
	return fmt.Sprintf("cmdline-%s-%s", p.Name, arch)
}

// MetalPlatform returns a metal platform.
func MetalPlatform() Platform {
	return Platform{
		Name: "metal",

		Label:       "Bare Metal",
		Description: "Runs on bare-metal servers",

		Architectures:   []Arch{ArchAmd64, ArchArm64},
		Documentation:   "/talos-guides/install/bare-metal-platforms/",
		DiskImageSuffix: "raw.zst",
		BootMethods: []BootMethod{
			BootMethodISO,
			BootMethodDiskImage,
			BootMethodPXE,
		},
	}
}

// CloudPlatforms returns a list of supported cloud platforms.
func CloudPlatforms() []Platform {
	// metal platform is not listed here, as it is handled separately.
	return []Platform{
		// Tier 1
		{
			Name: "aws",

			Label:       "Amazon Web Services (AWS)",
			Description: "Runs on AWS VMs booted from an AMI",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/aws/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "gcp",

			Label:       "Google Cloud (GCP)",
			Description: "Runs on Google Cloud VMs booted from a disk image",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/gcp/",
			DiskImageSuffix: "raw.tar.gz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "equinixMetal",

			Label:       "Equinix Metal",
			Description: "Runs on Equinix Metal bare-metal servers",

			Architectures: []Arch{ArchAmd64, ArchArm64},
			Documentation: "/talos-guides/install/bare-metal-platforms/equinix-metal/",
			BootMethods: []BootMethod{
				BootMethodPXE,
			},
		},
		// Tier 2
		{
			Name: "azure",

			Label:       "Microsoft Azure",
			Description: "Runs on Microsoft Azure Linux Virtual Machines",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/azure/",
			DiskImageSuffix: "vhd.xz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "digital-ocean",

			Label:       "Digital Ocean",
			Description: "Runs on Digital Ocean droplets",

			Architectures:   []Arch{ArchAmd64},
			Documentation:   "/talos-guides/install/cloud-platforms/digitalocean/",
			DiskImageSuffix: "raw.gz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "nocloud",

			Label:       "Nocloud",
			Description: "Runs on various hypervisors supporting 'nocloud' metadata (Proxmox, Oxide Computer, etc.)",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/nocloud/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
				BootMethodISO,
				BootMethodPXE,
			},
			SecureBootSupported: true,
		},
		{
			Name: "openstack",

			Label:       "OpenStack",
			Description: "Runs on OpenStack virtual machines",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/openstack/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
				BootMethodISO,
				BootMethodPXE,
			},
		},
		{
			Name: "vmware",

			Label:       "VMWare",
			Description: "Runs on VMWare ESXi virtual machines",

			Architectures:   []Arch{ArchAmd64},
			Documentation:   "/talos-guides/install/virtualized-platforms/vmware/",
			DiskImageSuffix: "ova",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
				BootMethodISO,
			},
		},
		// Tier 3
		{
			Name: "akamai",

			Label:       "Akamai",
			Description: "Runs on Akamai Cloud (Linode) virtual machines",

			Architectures:   []Arch{ArchAmd64},
			MinVersion:      semver.MustParse("1.7.0"),
			Documentation:   "/talos-guides/install/cloud-platforms/akamai/",
			DiskImageSuffix: "raw.gz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "cloudstack",

			Label:       "Apache CloudStack",
			Description: "Runs on Apache CloudStack virtual machines",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/cloudstack/",
			DiskImageSuffix: "raw.gz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
			MinVersion: semver.MustParse("1.8.0-alpha.2"),
		},
		{
			Name: "hcloud",

			Label:       "Hetzner",
			Description: "Runs on Hetzner virtual machines",

			Architectures:   []Arch{ArchAmd64},
			Documentation:   "/talos-guides/install/cloud-platforms/hetzner/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "oracle",

			Label:       "Oracle Cloud",
			Description: "Runs on Oracle Cloud virtual machines",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/oracle/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "upcloud",

			Label:       "UpCloud",
			Description: "Runs on UpCloud virtual machines",

			Architectures:   []Arch{ArchAmd64},
			Documentation:   "/talos-guides/install/cloud-platforms/ucloud/",
			DiskImageSuffix: "raw.xz",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
		},
		{
			Name: "vultr",

			Label:       "Vultr",
			Description: "Runs on Vultr Cloud Compute virtual machines",

			Architectures: []Arch{ArchAmd64},
			Documentation: "/talos-guides/install/cloud-platforms/vultr/",
			BootMethods: []BootMethod{
				BootMethodISO,
				BootMethodPXE,
			},
		},
		// Tier 4 (no documentation).
		{
			Name: "exoscale",

			Label:       "Exoscale",
			Description: "Runs on Exoscale virtual machines",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			Documentation:   "/talos-guides/install/cloud-platforms/exoscale/",
			DiskImageSuffix: "qcow2",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
			},
			MinVersion: semver.MustParse("1.3.0"),
		},
		{
			Name: "opennebula",

			Label:       "OpenNebula",
			Description: "Runs on OpenNebula virtual machines",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			DiskImageSuffix: "raw.zst",
			Documentation:   "/talos-guides/install/virtualized-platforms/opennebula/",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
				BootMethodISO,
			},
			MinVersion: semver.MustParse("1.7.0"),
		},
		{
			Name: "scaleway",

			Label:       "Scaleway",
			Description: "Runs on Scaleway virtual machines",

			Architectures:   []Arch{ArchAmd64, ArchArm64},
			DiskImageSuffix: "raw.zst",
			Documentation:   "/talos-guides/install/cloud-platforms/scaleway/",
			BootMethods: []BootMethod{
				BootMethodDiskImage,
				BootMethodISO,
			},
		},
	}
}
