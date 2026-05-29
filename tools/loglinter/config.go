// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import "github.com/siderolabs/talos/tools/loglinter/internal/loglinter"

// Config is the standalone CLI configuration type.
type Config = loglinter.Config

// LoadConfig reads and normalizes a standalone CLI configuration file.
func LoadConfig(path string) (Config, error) {
	return loglinter.LoadConfig(path)
}
