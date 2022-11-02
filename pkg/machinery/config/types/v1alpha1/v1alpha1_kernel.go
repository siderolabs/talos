// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/gen/slices"

	"github.com/siderolabs/talos/pkg/machinery/config"
)

// Modules implements config.Kernel interface.
func (kc *KernelConfig) Modules() []config.KernelModule {
	return slices.Map(kc.KernelModules, func(kmc *KernelModuleConfig) config.KernelModule { return kmc })
}

// Name implements config.KernelModule interface.
func (kmc *KernelModuleConfig) Name() string {
	return kmc.ModuleName
}

// Parameters implements config.KernelModule interface.
func (kmc *KernelModuleConfig) Parameters() []string {
	return kmc.ModuleParameters
}
