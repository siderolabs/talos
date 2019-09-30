/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package v1alpha1

import (
	"github.com/talos-systems/talos/pkg/config/cluster"
	"github.com/talos-systems/talos/pkg/config/machine"
)

// Config holds the full representation of the node config.
type Config struct {
	ConfigVersion string         `yaml:"version"`
	MachineConfig *MachineConfig `yaml:"machine"`
	ClusterConfig *ClusterConfig `yaml:"cluster"`
}

const (
	// Version is the version string for v1alpha1.
	Version = "v1alpha1"
)

// Version implements the Configurator interface.
func (n *Config) Version() string {
	return Version
}

// Debug implements the Configurator interface.
func (n *Config) Debug() bool {
	return false
}

// Machine implements the Configurator interface.
func (n *Config) Machine() machine.Machine {
	return n.MachineConfig
}

// Cluster implements the Configurator interface.
func (n *Config) Cluster() cluster.Cluster {
	return n.ClusterConfig
}

// Validate implements the Configurator interface.
func (n *Config) Validate() error {
	return nil
}

// String implements the Configurator interface.
func (n *Config) String() (string, error) {
	return "", nil
}
