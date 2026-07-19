// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type criBaseRuntimeSpecConfigShim struct {
	overrides map[string]any
}

func (s criBaseRuntimeSpecConfigShim) Overrides() map[string]any {
	return s.overrides
}

func (s criBaseRuntimeSpecConfigShim) CRIBaseRuntimeSpecConfigSignal() {}

// CRIBaseRuntimeSpecConfig implements the config.Config interface.
func (c *Config) CRIBaseRuntimeSpecConfig() config.CRIBaseRuntimeSpecConfig {
	if c == nil || c.MachineConfig == nil || len(c.MachineConfig.MachineBaseRuntimeSpecOverrides.Object) == 0 { //nolint:staticcheck // compatibility with deprecated configuration
		return nil
	}

	return criBaseRuntimeSpecConfigShim{
		overrides: c.MachineConfig.MachineBaseRuntimeSpecOverrides.Object, //nolint:staticcheck // compatibility with deprecated configuration
	}
}

type criCustomizationConfigShim struct {
	content string
}

func (criCustomizationConfigShim) Name() string {
	return config.LegacyCRICustomizationConfigName
}

func (s criCustomizationConfigShim) Content() string {
	return s.content
}

func (s criCustomizationConfigShim) CRICustomizationConfigSignal() {}

// CRICustomizationConfigs implements the config.Config interface.
func (c *Config) CRICustomizationConfigs() []config.CRICustomizationConfig {
	if c == nil || c.MachineConfig == nil {
		return nil
	}

	legacyPath := filepath.Join("/etc", constants.CRICustomizationConfigPart)

	for _, file := range c.MachineConfig.MachineFiles { //nolint:staticcheck // compatibility with deprecated configuration
		if file != nil && file.FilePath == legacyPath {
			return []config.CRICustomizationConfig{
				criCustomizationConfigShim{content: file.Content()},
			}
		}
	}

	return nil
}
