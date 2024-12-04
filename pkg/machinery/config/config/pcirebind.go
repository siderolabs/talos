// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// PCIRebindConfig defines the interface to access PCI rebind configuration.
type PCIRebindConfig interface {
	PCIRebindConfigs() []PCIRebindConfigDriver
}

// PCIRebindConfigDriver defines the interface to access PCI rebind configuration.
type PCIRebindConfigDriver interface {
	Name() string
	VendorDeviceID() string
	TargetDriver() string
}

// WrapPCIRebindConfig wraps a list of PCIRebindConfig into a single PCIRebindConfig aggregating the results.
func WrapPCIRebindConfig(configs ...PCIRebindConfig) PCIRebindConfig {
	return pciRebindConfigWrapper(configs)
}

type pciRebindConfigWrapper []PCIRebindConfig

func (w pciRebindConfigWrapper) PCIRebindConfigs() []PCIRebindConfigDriver {
	return aggregateValues(w, func(c PCIRebindConfig) []PCIRebindConfigDriver {
		return c.PCIRebindConfigs()
	})
}
