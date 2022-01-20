// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config specifies Talos installer extensions configuration.
type Config struct {
	Layers []*Layer `yaml:"layers"`
}

// Layer defines overlay mount layer.
type Layer struct {
	Image    string   `yaml:"image"`
	Metadata Metadata `yaml:"metadata"`
}

// Read extensions config from a file.
func (cfg *Config) Read(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return yaml.NewDecoder(f).Decode(cfg)
}

// Write extensions config to a file.
func (cfg *Config) Write(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	return yaml.NewEncoder(f).Encode(cfg)
}
