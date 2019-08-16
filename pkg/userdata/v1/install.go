/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package v1 provides user-facing v1 machine configs
//nolint: dupl
package v1

// Install represents the installation options for preparing a node.
type Install struct {
	Boot            *BootDevice    `yaml:"boot,omitempty"`
	Ephemeral       *InstallDevice `yaml:"ephemeral,omitempty"`
	ExtraDevices    []*ExtraDevice `yaml:"extraDevices,omitempty"`
	ExtraKernelArgs []string       `yaml:"extraKernelArgs,omitempty"`
	Wipe            bool           `yaml:"wipe"`
	Force           bool           `yaml:"force"`
}

// BootDevice represents the install options specific to the boot partition.
type BootDevice struct {
	InstallDevice `yaml:",inline"`

	Kernel    string `yaml:"kernel"`
	Initramfs string `yaml:"initramfs"`
}

// RootDevice represents the install options specific to the root partition.
type RootDevice struct {
	InstallDevice `yaml:",inline"`

	Rootfs string `yaml:"rootfs"`
}

// InstallDevice represents the specific directions for each partition.
type InstallDevice struct {
	Device string `yaml:"device,omitempty"`
	Size   uint   `yaml:"size,omitempty"`
}

// ExtraDevice represents the options available for partitioning, formatting,
// and mounting extra disks.
type ExtraDevice struct {
	Device     string                  `yaml:"device,omitempty"`
	Partitions []*ExtraDevicePartition `yaml:"partitions,omitempty"`
}

// ExtraDevicePartition represents the options for a device partition.
type ExtraDevicePartition struct {
	Size       uint   `yaml:"size,omitempty"`
	MountPoint string `yaml:"mountpoint,omitempty"`
}
