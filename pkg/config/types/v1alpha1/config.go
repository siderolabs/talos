// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/internal/pkg/runtime"
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
func (c *Config) Version() string {
	return Version
}

// Debug implements the Configurator interface.
func (c *Config) Debug() bool {
	return false
}

// Machine implements the Configurator interface.
func (c *Config) Machine() machine.Machine {
	return c.MachineConfig
}

// Cluster implements the Configurator interface.
func (c *Config) Cluster() cluster.Cluster {
	return c.ClusterConfig
}

// Validate implements the Configurator interface.
func (c *Config) Validate(mode runtime.Mode) error {
	if c.MachineConfig == nil {
		return errors.New("machine instructions are required")
	}

	if c.ClusterConfig == nil {
		return errors.New("cluster instructions are required")
	}

	if c.Cluster().Endpoint() == nil || c.Cluster().Endpoint().String() == "" {
		return errors.New("a cluster endpoint is required")
	}

	if mode == runtime.Metal {
		if c.MachineConfig.MachineInstall == nil {
			return fmt.Errorf("install instructions are required by the %q mode", runtime.Metal.String())
		}
	}

	return nil
}

// String implements the Configurator interface.
func (c *Config) String() (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
