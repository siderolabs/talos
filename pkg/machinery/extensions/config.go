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
	Image string `yaml:"image"`
}

// LoadConfig load extensions config from a file.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	var extensions *Config

	decoder := yaml.NewDecoder(f)
	if err = decoder.Decode(&extensions); err != nil {
		return nil, err
	}

	return extensions, nil
}
