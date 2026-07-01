// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Name implements config.KernelModuleConfig interface.
func (cfg *KernelModuleConfig) Name() string {
	return cfg.ModuleName
}

// Parameters implements config.KernelModuleConfig interface.
func (cfg *KernelModuleConfig) Parameters() []string {
	return cfg.ModuleParameters
}

// KernelModuleConfigs returns the kernel module configs derived from the legacy v1alpha1 machine kernel config.
func (c *Config) KernelModuleConfigs() []config.KernelModuleConfig {
	if c.MachineConfig == nil || c.MachineConfig.MachineKernel == nil { //nolint:staticcheck // legacy configuration
		return nil
	}

	return xslices.Map(c.MachineConfig.MachineKernel.KernelModules, func(m *KernelModuleConfig) config.KernelModuleConfig { //nolint:staticcheck // legacy configuration
		return m
	})
}
