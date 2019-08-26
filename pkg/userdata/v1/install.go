/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package v1 provides user-facing v1 machine configs
//nolint: dupl
package v1

// Install represents the installation options for preparing a node.
type Install struct {
	Disk            string       `yaml:"disk,omitempty"`
	ExtraDisks      []*ExtraDisk `yaml:"extraDisks,omitempty"`
	ExtraKernelArgs []string     `yaml:"extraKernelArgs,omitempty"`
	Image           string       `yaml:"image,omitempty"`
	Bootloader      bool         `yaml:"bootloader,omitempty"`
	Wipe            bool         `yaml:"wipe"`
	Force           bool         `yaml:"force"`
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
