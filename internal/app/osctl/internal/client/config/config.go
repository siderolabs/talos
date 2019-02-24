/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package config

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

// Config represents the configuration file.
type Config struct {
	Context  string              `yaml:"context"`
	Contexts map[string]*Context `yaml:"contexts"`
}

// Context represents the set of credentials required to talk to a target.
type Context struct {
	Target string `yaml:"target"`
	CA     string `yaml:"ca"`
	Crt    string `yaml:"crt"`
	Key    string `yaml:"key"`
}

// Open reads the config and initilzes a Config struct.
func Open(p string) (c *Config, err error) {
	fileBytes, err := ioutil.ReadFile(p)
	if err != nil {
		return
	}

	c = &Config{}
	if err = yaml.Unmarshal(fileBytes, c); err != nil {
		return
	}

	return c, nil
}

// Save writes the config to disk.
func (c *Config) Save(p string) (err error) {
	configBytes, err := yaml.Marshal(c)
	if err != nil {
		return
	}

	if err = ioutil.WriteFile(p, configBytes, 0600); err != nil {
		return
	}

	return nil
}
