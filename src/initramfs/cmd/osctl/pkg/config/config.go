package config

import (
	"io/ioutil"
	"os/user"
	"path"

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
func Open() (c *Config, err error) {
	u, err := user.Current()
	if err != nil {
		return
	}
	fileBytes, err := ioutil.ReadFile(path.Join(u.HomeDir, ".talos", "config"))
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
func (c *Config) Save() (err error) {
	u, err := user.Current()
	if err != nil {
		return
	}
	configBytes, err := yaml.Marshal(c)
	if err != nil {
		return
	}

	if err = ioutil.WriteFile(path.Join(u.HomeDir, ".talos", "config"), configBytes, 0600); err != nil {
		return
	}

	return nil
}
