// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// Modules implements config.Kernel interface.
func (kc *KernelConfig) Modules() []config.KernelModule {
	if kc.KernelModules == nil {
		return nil
	}

	res := make([]config.KernelModule, len(kc.KernelModules))
	for i, m := range kc.KernelModules {
		res[i] = config.KernelModule(m)
	}

	return res
}

// Name implements config.KernelModule interface.
func (kmc *KernelModuleConfig) Name() string {
	return kmc.ModuleName
}
