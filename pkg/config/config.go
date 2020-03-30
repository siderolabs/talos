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

	"github.com/talos-systems/talos/cmd/talosctl/pkg/client/config"
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

// MachineConfigs is an array of machine configs
type MachineConfigs struct {
	MachineConfigs map[string]v1alpha1.MachineConfig `yaml:"machineConfigs"`
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

	var input *generate.Input2

	input, err := generate.NewInput2(
		options.InputOptions.ClusterName,
		options.InputOptions.Endpoint,
		options.InputOptions.KubeVersion,
		options.InputOptions.GenOptions...,
	)
	if err != nil {
		return bundle, err
	}

	// generate base configs per node type
	for _, configType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
		var generatedConfig *v1alpha1.Config

		generatedConfig, err = generate.BaseConfig(configType, input)
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

	/////////// TODO: MOVE THIS LOGIC /////////////

	// generate host configs
	configsFile := "C:\\Users\\dave.CABITO\\go\\src\\dave93cab\\talos\\template\\MachineConfigs.yaml"

	// read configs
	data, err := ioutil.ReadFile(configsFile)
	if err != nil {
		return bundle, err
		// return fmt.Errorf("could not read file: %w", configsFile)
	}

	// unmarshall as array of machine configs
	machineConfigs := MachineConfigs{}

	err = yaml.Unmarshal([]byte(data), &machineConfigs)
	if err != nil {
		return bundle, err
		// return fmt.Errorf("could not parse machine config yaml")
	}

	bundle.HostCfgs = make(map[string]*v1alpha1.Config)

	for machineID, hostConfig := range machineConfigs.MachineConfigs {

		// choose base config to modify
		var config v1alpha1.Config
		switch hostConfig.MachineType {
		case "init":
			config = *bundle.InitCfg
		case "controlplane":
			config = *bundle.ControlPlaneCfg
		case "join":
			config = *bundle.JoinCfg
		default:
			return bundle, err
			// return fmt.Errorf("invalid machine type %s for machine %s", hostConfig.MachineType, machineID)
		}

		// overwrite with
		if hostConfig.MachineNetwork != nil {
			config.MachineConfig.MachineNetwork = hostConfig.MachineNetwork
		}
		if hostConfig.MachineInstall != nil {
			config.MachineConfig.MachineInstall = hostConfig.MachineInstall
		}
		if hostConfig.MachineCertSANs != nil {
			config.MachineConfig.MachineCertSANs = hostConfig.MachineCertSANs
		}
		if hostConfig.MachineKubelet != nil {
			config.MachineConfig.MachineKubelet = hostConfig.MachineKubelet
		}

		bundle.HostCfgs[machineID] = &config
	}

	// generate talos config
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
