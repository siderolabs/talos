// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"gopkg.in/yaml.v3"

	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configpatcher"
	"github.com/talos-systems/talos/pkg/machinery/config/encoder"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// ConfigBundle defines the group of v1alpha1 config files.
// docgen: nodoc
type ConfigBundle struct {
	InitCfg         *Config
	ControlPlaneCfg *Config
	JoinCfg         *Config
	TalosCfg        *clientconfig.Config
}

// Init implements the ConfiguratorBundle interface.
func (c *ConfigBundle) Init() config.Provider {
	return c.InitCfg
}

// ControlPlane implements the ConfiguratorBundle interface.
func (c *ConfigBundle) ControlPlane() config.Provider {
	return c.ControlPlaneCfg
}

// Join implements the ConfiguratorBundle interface.
func (c *ConfigBundle) Join() config.Provider {
	return c.JoinCfg
}

// TalosConfig implements the ConfiguratorBundle interface.
func (c *ConfigBundle) TalosConfig() *clientconfig.Config {
	return c.TalosCfg
}

// Write config files to output directory.
func (c *ConfigBundle) Write(outputDir string, commentsFlags encoder.CommentsFlags, types ...machine.Type) error {
	for _, t := range types {
		name := strings.ToLower(t.String()) + ".yaml"
		fullFilePath := filepath.Join(outputDir, name)

		var (
			configString string
			err          error
		)

		switch t { //nolint:exhaustive
		case machine.TypeInit:
			configString, err = c.Init().String(encoder.WithComments(commentsFlags))
			if err != nil {
				return err
			}
		case machine.TypeControlPlane:
			configString, err = c.ControlPlane().String(encoder.WithComments(commentsFlags))
			if err != nil {
				return err
			}
		case machine.TypeJoin:
			configString, err = c.Join().String(encoder.WithComments(commentsFlags))
			if err != nil {
				return err
			}
		}

		if err = ioutil.WriteFile(fullFilePath, []byte(configString), 0o644); err != nil {
			return err
		}

		fmt.Printf("created %s\n", fullFilePath)
	}

	return nil
}

// ApplyJSONPatch patches every config type with a patch.
func (c *ConfigBundle) ApplyJSONPatch(patch jsonpatch.Patch) error {
	if len(patch) == 0 {
		return nil
	}

	apply := func(in *Config) (out *Config, err error) {
		var marshaled []byte

		marshaled, err = in.Bytes()
		if err != nil {
			return nil, err
		}

		var patched []byte

		patched, err = configpatcher.JSON6902(marshaled, patch)
		if err != nil {
			return nil, err
		}

		out = &Config{}
		err = yaml.Unmarshal(patched, out)

		return out, err
	}

	var err error

	c.InitCfg, err = apply(c.InitCfg)
	if err != nil {
		return err
	}

	c.ControlPlaneCfg, err = apply(c.ControlPlaneCfg)
	if err != nil {
		return err
	}

	c.JoinCfg, err = apply(c.JoinCfg)
	if err != nil {
		return err
	}

	return nil
}
