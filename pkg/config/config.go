// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
)

// Content represents the raw config data.
type Content struct {
	Version string `yaml:"version"`

	data []byte
}

// NewConfigBundle returns a new bundle
// nolint: gocyclo
func NewConfigBundle(opts ...BundleOption) (*v1alpha1.ConfigBundle, error) {
	options := DefaultBundleOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	bundle := &v1alpha1.ConfigBundle{}

	// Configs already exist, we'll pull them in.
	if options.ExistingConfigs != "" {
		if options.InputOptions != nil {
			return bundle, fmt.Errorf("both existing config path and input options specified")
		}

		// Pull existing machine configs of each type
		for _, configType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
			data, err := ioutil.ReadFile(filepath.Join(options.ExistingConfigs, strings.ToLower(configType.String())+".yaml"))
			if err != nil {
				return bundle, err
			}

			unmarshalledConfig := &v1alpha1.Config{}
			if err := yaml.Unmarshal(data, unmarshalledConfig); err != nil {
				return bundle, err
			}

			switch configType {
			case machine.TypeInit:
				bundle.InitCfg = unmarshalledConfig
			case machine.TypeControlPlane:
				bundle.ControlPlaneCfg = unmarshalledConfig
			case machine.TypeWorker:
				bundle.JoinCfg = unmarshalledConfig
			}
		}

		// Pull existing talosconfig
		talosConfig, err := ioutil.ReadFile(filepath.Join(options.ExistingConfigs, "talosconfig"))
		if err != nil {
			return bundle, err
		}

		bundle.TalosCfg = &config.Config{}
		if err = yaml.Unmarshal(talosConfig, bundle.TalosCfg); err != nil {
			return bundle, err
		}

		return bundle, nil
	}

	// Handle generating net-new configs
	fmt.Println("generating PKI and tokens")

	var input *generate.Input

	input, err := generate.NewInput(
		options.InputOptions.ClusterName,
		options.InputOptions.Endpoint,
		options.InputOptions.KubeVersion,
		options.InputOptions.GenOptions...,
	)
	if err != nil {
		return bundle, err
	}

	for _, configType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
		var generatedConfig *v1alpha1.Config

		generatedConfig, err = generate.Config(configType, input)
		if err != nil {
			return bundle, err
		}

		switch configType {
		case machine.TypeInit:
			bundle.InitCfg = generatedConfig
		case machine.TypeControlPlane:
			bundle.ControlPlaneCfg = generatedConfig
		case machine.TypeWorker:
			bundle.JoinCfg = generatedConfig
		}
	}

	bundle.TalosCfg, err = generate.Talosconfig(input, options.InputOptions.GenOptions...)
	if err != nil {
		return bundle, err
	}

	return bundle, nil
}

// newConfig initializes and returns a Configurator.
func newConfig(c Content) (config runtime.Configurator, err error) {
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

// NewFromFile will take a filepath and attempt to parse a config file from it
func NewFromFile(filepath string) (runtime.Configurator, error) {
	content, err := fromFile(filepath)
	if err != nil {
		return nil, err
	}

	return newConfig(content)
}

// NewFromBytes will take a byteslice and attempt to parse a config file from it
func NewFromBytes(in []byte) (runtime.Configurator, error) {
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
