/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

// Install represents the installation options for preparing a node.
type Install struct {
	Disk            string         `yaml:"disk,omitempty"`
	ExtraDevices    []*ExtraDevice `yaml:"extraDevices,omitempty"`
	ExtraKernelArgs []string       `yaml:"extraKernelArgs,omitempty"`
	Image           string         `yaml:"image,omitempty"`
	Bootloader      bool           `yaml:"bootloader,omitempty"`
	Wipe            bool           `yaml:"wipe"`
	Force           bool           `yaml:"force"`
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
