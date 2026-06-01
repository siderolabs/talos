// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// PCIDriverRebindConfig defines the interface to access PCI rebind configuration.
type PCIDriverRebindConfig interface {
	PCIDriverRebindConfigs() []PCIDriverRebindConfigDriver
}

// PCIDriverRebindConfigDriver defines the interface to access PCI rebind configuration.
type PCIDriverRebindConfigDriver interface {
	PCIID() string
	TargetDriver() string
}

// WrapPCIDriverRebindConfig wraps a list of PCIDriverRebindConfig into a single PCIDriverRebindConfig aggregating the results.
func WrapPCIDriverRebindConfig(configs ...PCIDriverRebindConfig) PCIDriverRebindConfig {
	return pciDriverRebindConfigWrapper(configs)
}

type pciDriverRebindConfigWrapper []PCIDriverRebindConfig

func (w pciDriverRebindConfigWrapper) PCIDriverRebindConfigs() []PCIDriverRebindConfigDriver {
	return aggregateValues(w, func(c PCIDriverRebindConfig) []PCIDriverRebindConfigDriver {
		return c.PCIDriverRebindConfigs()
	})
}
