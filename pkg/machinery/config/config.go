// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package config provides methods to generate and consume Talos configuration.
package config

//go:generate docgen -generate-schema-from-dir types/ -json-schema-output schemas/config.schema.json -version-tag-file ../gendata/data/tag

import "github.com/siderolabs/talos/pkg/machinery/config/config"

// Config defines the interface to access contents of the machine configuration.
type Config = config.Config
