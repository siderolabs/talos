// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configloader provides methods to load Talos config.
package configloader

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

// content represents the raw config data.
//
//docgen: nodoc
type content struct {
	Version string `yaml:"version"`

	data []byte
}

// newConfig initializes and returns a Configurator.
func newConfig(c content) (config config.Provider, err error) {
	switch c.Version {
	case v1alpha1.Version:
		return v1alpha1.Load(c.data)
	default:
		return nil, fmt.Errorf("unknown version: %q", c.Version)
	}
}

// NewFromFile will take a filepath and attempt to parse a config file from it.
func NewFromFile(filepath string) (config.Provider, error) {
	c, err := fromFile(filepath)
	if err != nil {
		return nil, err
	}

	return newConfig(c)
}

// NewFromBytes will take a byteslice and attempt to parse a config file from it.
func NewFromBytes(in []byte) (config.Provider, error) {
	c, err := fromBytes(in)
	if err != nil {
		return nil, err
	}

	return newConfig(c)
}

// fromFile is a convenience function that reads the config from disk, and
// unmarshals it.
func fromFile(p string) (c content, err error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}

	return unmarshal(b)
}

// fromBytes is a convenience function that reads the config from a string, and
// unmarshals it.
func fromBytes(b []byte) (c content, err error) {
	return unmarshal(b)
}

func unmarshal(b []byte) (c content, err error) {
	c = content{
		data: b,
	}

	if err = yaml.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("failed to parse config: %s", err.Error())
	}

	return c, nil
}
