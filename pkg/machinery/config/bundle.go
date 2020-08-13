// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config provides methods to generate and consume Talos configuration.
package config

import "github.com/talos-systems/talos/pkg/machinery/client/config"

// ProviderBundle defines the configuration bundle interface.
type ProviderBundle interface {
	Init() Provider
	ControlPlane() Provider
	Join() Provider
	TalosConfig() *config.Config
}
