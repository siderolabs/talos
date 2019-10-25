// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

// Content represents the raw config data.
type Content struct {
	Version string `yaml:"version"`

	data []byte
}

// New initializes and returns a Configurator.
func New(c Content) (config runtime.Configurator, err error) {
	switch c.Version {
	case v1alpha1.Version:
		config = &v1alpha1.Config{}
		if err = yaml.Unmarshal(c.data, config); err != nil {
			return config, fmt.Errorf("failed to parse version: %w", err)
		}

		return config, nil
	default:
		return nil, fmt.Errorf("unknown version: %q", c.Version)
	}
}

// FromFile is a convenience function that reads the config from disk, and
// unmarshals it.
func FromFile(p string) (c Content, err error) {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return c, fmt.Errorf("read config: %w", err)
	}

	return unmarshal(b)
}

// FromBytes is a convenience function that reads the config from a string, and
// unmarshals it.
func FromBytes(b []byte) (c Content, err error) {
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
