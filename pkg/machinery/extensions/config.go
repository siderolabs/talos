// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"os"

	"go.yaml.in/yaml/v4"
)

//go:generate go tool github.com/siderolabs/deep-copy -type Layer -header-file ../../../hack/boilerplate.txt -o deep_copy.generated.go .

// Config specifies Talos installer extensions configuration.
type Config struct {
	Layers []*Layer `yaml:"layers"`
}

// Layer defines overlay mount layer.
//
//gotagsrewrite:gen
type Layer struct {
	Image    string   `yaml:"image" protobuf:"1"`
	Metadata Metadata `yaml:"metadata" protobuf:"2"`
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
