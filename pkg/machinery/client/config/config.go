// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/siderolabs/crypto/x509"
	"gopkg.in/yaml.v3"
)

// Config represents the client configuration file (talosconfig).
type Config struct {
	Context  string              `yaml:"context"`
	Contexts map[string]*Context `yaml:"contexts"`

	// path is the config Path config is read from.
	path Path
}

// NewConfig returns the client configuration file with a single context.
func NewConfig(contextName string, endpoints []string, caCrt []byte, client *x509.PEMEncodedCertificateAndKey) *Config {
	return &Config{
		Context: contextName,
		Contexts: map[string]*Context{
			contextName: {
				Endpoints: endpoints,
				CA:        base64.StdEncoding.EncodeToString(caCrt),
				Crt:       base64.StdEncoding.EncodeToString(client.Crt),
				Key:       base64.StdEncoding.EncodeToString(client.Key),
			},
		},
	}
}

func (c *Config) upgrade() {
	for _, ctx := range c.Contexts {
		ctx.upgrade()
	}
}

// Context represents the set of credentials required to talk to a target.
type Context struct {
	DeprecatedTarget string   `yaml:"target,omitempty"` // Field deprecated in favor of Endpoints
	Endpoints        []string `yaml:"endpoints"`
	Nodes            []string `yaml:"nodes,omitempty"`
	CA               string   `yaml:"ca,omitempty"`
	Crt              string   `yaml:"crt,omitempty"`
	Key              string   `yaml:"key,omitempty"`
	Auth             Auth     `yaml:"auth,omitempty"`
	Cluster          string   `yaml:"cluster,omitempty"`
}

// Auth may hold credentials for an authentication method such as Basic Auth.
type Auth struct {
	Basic    *Basic    `yaml:"basic,omitempty"`
	SideroV1 *SideroV1 `yaml:"siderov1,omitempty"`
}

// Basic holds Basic Auth credentials.
type Basic struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// SideroV1 holds information for SideroV1 API signature auth.
type SideroV1 struct {
	Identity string `yaml:"identity"`
}

func (c *Context) upgrade() {
	if c.DeprecatedTarget != "" {
		c.Endpoints = append(c.Endpoints, c.DeprecatedTarget)
		c.DeprecatedTarget = ""
	}
}

// Open reads the config and initializes a Config struct.
// If path is explicitly set, it will be used.
// If not, the default path rules will be used.
func Open(path string) (*Config, error) {
	var (
		confPath Path
		err      error
	)

	if path != "" { // path is explicitly specified, ensure that is created and use it
		confPath = Path{
			Path:         path,
			WriteAllowed: true,
		}

		err = ensure(confPath.Path)
		if err != nil {
			return nil, err
		}
	} else { // path is implicit, get the first already existing & readable path or ensure that it is created
		confPath, err = firstValidPath()
		if err != nil {
			return nil, err
		}
	}

	config, err := fromFile(confPath.Path)
	if err != nil {
		return nil, err
	}

	config.path = confPath

	return config, nil
}

func fromFile(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close() //nolint:errcheck

	return ReadFrom(file)
}

// FromString returns a config from a string.
func FromString(p string) (c *Config, err error) {
	return ReadFrom(bytes.NewReader([]byte(p)))
}

// FromBytes returns a config from []byte.
func FromBytes(b []byte) (c *Config, err error) {
	return ReadFrom(bytes.NewReader(b))
}

// ReadFrom reads a config from io.Reader.
func ReadFrom(r io.Reader) (c *Config, err error) {
	c = &Config{}

	if err = yaml.NewDecoder(r).Decode(c); err != nil {
		return
	}

	c.upgrade()

	return
}

// Save writes the config to disk.
// If the path is not explicitly set, the default path rules will be used.
func (c *Config) Save(path string) error {
	var err error

	if path != "" { // path is explicitly specified, use it
		c.path = Path{
			Path:         path,
			WriteAllowed: true,
		}
	} else if c.path.Path == "" { // path is implicit and is not set on config, get the first already existing & writable path or create it
		c.path, err = firstValidPath()
		if err != nil {
			return err
		}
	}

	if !c.path.WriteAllowed {
		return fmt.Errorf("not allowed to write to config: %s", c.path.Path)
	}

	configBytes, err := c.Bytes()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(c.path.Path), 0o700); err != nil {
		return err
	}

	return os.WriteFile(c.path.Path, configBytes, 0o600)
}

// Bytes gets yaml encoded config data.
func (c *Config) Bytes() ([]byte, error) {
	return yaml.Marshal(c)
}

// Path returns the filesystem path config was read from.
func (c *Config) Path() Path {
	return c.path
}

// Rename describes context rename during merge.
type Rename struct {
	From string
	To   string
}

// String converts to "from" -> "to".
func (r *Rename) String() string {
	return fmt.Sprintf("%q -> %q", r.From, r.To)
}

// Merge in additional contexts from another Config.
//
// Current context is overridden from passed in config.
func (c *Config) Merge(cfg *Config) []Rename {
	if c.Contexts == nil {
		c.Contexts = map[string]*Context{}
	}

	mappedContexts := map[string]string{}
	renames := []Rename{}

	for name, ctx := range cfg.Contexts {
		mergedName := name //nolint:copyloopvar

		if _, exists := c.Contexts[mergedName]; exists {
			for i := 1; ; i++ {
				mergedName = fmt.Sprintf("%s-%d", name, i)

				if _, exists := c.Contexts[mergedName]; !exists {
					break
				}
			}
		}

		mappedContexts[name] = mergedName

		if name != mergedName {
			renames = append(renames, Rename{name, mergedName})
		}

		c.Contexts[mergedName] = ctx
	}

	if cfg.Context != "" {
		c.Context = mappedContexts[cfg.Context]
	}

	return renames
}

func ensure(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		config := &Config{
			Context:  "",
			Contexts: map[string]*Context{},
		}

		return config.Save(path)
	}

	return nil
}
