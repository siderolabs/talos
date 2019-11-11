// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package v1alpha1 provides user-facing v1alpha1 machine configs
//nolint: dupl
package v1alpha1

// InstallConfig represents the installation options for preparing a node.
type InstallConfig struct {
	InstallDisk            string   `yaml:"disk,omitempty"`
	InstallExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	InstallImage           string   `yaml:"image,omitempty"`
	InstallBootloader      bool     `yaml:"bootloader,omitempty"`
	InstallWipe            bool     `yaml:"wipe"`
	InstallForce           bool     `yaml:"force"`
}

// Image implements the Configurator interface.
func (i *InstallConfig) Image() string {
	return i.InstallImage
}

// Disk implements the Configurator interface.
func (i *InstallConfig) Disk() string {
	return i.InstallDisk
}

// ExtraKernelArgs implements the Configurator interface.
func (i *InstallConfig) ExtraKernelArgs() []string {
	return i.InstallExtraKernelArgs
}

// Zero implements the Configurator interface.
func (i *InstallConfig) Zero() bool {
	return i.InstallWipe
}

// Force implements the Configurator interface.
func (i *InstallConfig) Force() bool {
	return i.InstallForce
}

// WithBootloader implements the Configurator interface.
func (i *InstallConfig) WithBootloader() bool {
	return i.InstallBootloader
}
