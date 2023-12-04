// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// ExtensionServicesConfigConfig is a config for extension services.
type ExtensionServicesConfigConfig interface {
	ConfigData() []ExtensionServicesConfig
}

// ExtensionServicesConfig is a config for extension services.
type ExtensionServicesConfig interface {
	Name() string
	ConfigFiles() []ExtensionServicesConfigFile
}

// ExtensionServicesConfigFile is a config file for extension services.
type ExtensionServicesConfigFile interface {
	Content() string
	Path() string
}
