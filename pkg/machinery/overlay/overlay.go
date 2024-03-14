// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package overlay provides an interface for overlay installers.
package overlay

// Installer is an interface for overlay installers.
type Installer[T any] interface {
	GetOptions(extra T) (Options, error)
	Install(options InstallOptions[T]) error
}

// Options for the overlay installer.
type Options struct {
	Name             string           `yaml:"name"`
	KernelArgs       []string         `yaml:"kernelArgs,omitempty"`
	PartitionOptions PartitionOptions `yaml:"partitionOptions,omitempty"`
}

// PartitionOptions for the overlay installer.
type PartitionOptions struct {
	Offset uint64 `yaml:"offset,omitempty"`
}

// InstallOptions for the overlay installer.
type InstallOptions[T any] struct {
	InstallDisk   string `yaml:"installDisk"`
	MountPrefix   string `yaml:"mountPrefix"`
	ArtifactsPath string `yaml:"artifactsPath"`
	ExtraOptions  T      `yaml:"extraOptions,omitempty"`
}

// ExtraOptions for the overlay installer.
type ExtraOptions map[string]any
