// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/pkg/config"
)

// Content represents the raw config data.
type Content struct {
	Version string `yaml:"version"`

	data []byte
}

// newConfig initializes and returns a Configurator.
func newConfig(c Content) (config *Config, err error) {
	switch c.Version {
	case Version:
		config = &Config{}
		if err = yaml.Unmarshal(c.data, config); err != nil {
			return config, fmt.Errorf("failed to parse version: %w", err)
		}

		return config, nil
	default:
		return nil, fmt.Errorf("unknown version: %q", c.Version)
	}
}

// NewFromFile will take a filepath and attempt to parse a config file from it.
func NewFromFile(filepath string) (config.Provider, error) {
	content, err := fromFile(filepath)
	if err != nil {
		return nil, err
	}

	return newConfig(content)
}

// NewFromBytes will take a byteslice and attempt to parse a config file from it.
func NewFromBytes(in []byte) (config.Provider, error) {
	content, err := fromBytes(in)
	if err != nil {
		return nil, err
	}

	return newConfig(content)
}

// fromFile is a convenience function that reads the config from disk, and
// unmarshals it.
func fromFile(p string) (c Content, err error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}

	return unmarshal(b)
}

// fromBytes is a convenience function that reads the config from a string, and
// unmarshals it.
func fromBytes(b []byte) (c Content, err error) {
	return unmarshal(b)
}

func unmarshal(b []byte) (c Content, err error) {
	c = Content{
		data: b,
	}

	if err = yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("failed to parse config: %s", err.Error())
	}

	return c, nil
}
