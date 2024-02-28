// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package overlay provides an interface for overlay installers.
package overlay

// Installer is an interface for overlay installers.
type Installer interface {
	GetOptions(extra InstallExtraOptions) (Options, error)
	Install(options InstallOptions) error
}

// Options for the overlay installer.
type Options struct {
	Name             string   `yaml:"name"`
	KernelArgs       []string `yaml:"kernelArgs,omitempty"`
	PartitionOptions struct {
		Offset uint64
	} `yaml:"partitionOptions,omitempty"`
}

// InstallOptions for the overlay installer.
type InstallOptions struct {
	InstallDisk   string              `yaml:"installDisk"`
	MountPrefix   string              `yaml:"mountPrefix"`
	ArtifactsPath string              `yaml:"artifactsPath"`
	ExtraOptions  InstallExtraOptions `yaml:"extraOptions,omitempty"`
}

// InstallExtraOptions for the overlay installer.
type InstallExtraOptions map[string]any
