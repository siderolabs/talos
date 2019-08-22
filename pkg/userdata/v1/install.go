/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package v1 provides user-facing v1 machine configs
//nolint: dupl
package v1

// Install represents the installation options for preparing a node.
type Install struct {
	Boot            *BootDisk    `yaml:"boot,omitempty"`
	Ephemeral       *InstallDisk `yaml:"ephemeral,omitempty"`
	ExtraDisks      []*ExtraDisk `yaml:"extraDisks,omitempty"`
	ExtraKernelArgs []string     `yaml:"extraKernelArgs,omitempty"`
	Wipe            bool         `yaml:"wipe"`
	Force           bool         `yaml:"force"`
}

// BootDisk represents the install options specific to the boot partition.
type BootDisk struct {
	InstallDisk `yaml:",inline"`

	Kernel    string `yaml:"kernel"`
	Initramfs string `yaml:"initramfs"`
}

// RootDisk represents the install options specific to the root partition.
type RootDisk struct {
	InstallDisk `yaml:",inline"`

	Rootfs string `yaml:"rootfs"`
}

// InstallDisk represents the specific directions for each partition.
type InstallDisk struct {
	Disk string `yaml:"disk,omitempty"`
	Size uint   `yaml:"size,omitempty"`
}

// ExtraDisk represents the options available for partitioning, formatting,
// and mounting extra disks.
type ExtraDisk struct {
	Disk       string                `yaml:"disk,omitempty"`
	Partitions []*ExtraDiskPartition `yaml:"partitions,omitempty"`
}

// ExtraDiskPartition represents the options for a device partition.
type ExtraDiskPartition struct {
	Size       uint   `yaml:"size,omitempty"`
	MountPoint string `yaml:"mountpoint,omitempty"`
}
