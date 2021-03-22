// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bundle

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// NewConfigBundle returns a new bundle.
//nolint:gocyclo,cyclop
func NewConfigBundle(opts ...Option) (*v1alpha1.ConfigBundle, error) {
	options := DefaultOptions()

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
		for _, configType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeJoin} {
			data, err := ioutil.ReadFile(filepath.Join(options.ExistingConfigs, strings.ToLower(configType.String())+".yaml"))
			if err != nil {
				return bundle, err
			}

			unmarshalledConfig := &v1alpha1.Config{}
			if err := yaml.Unmarshal(data, unmarshalledConfig); err != nil {
				return bundle, err
			}

			switch configType { //nolint:exhaustive
			case machine.TypeInit:
				bundle.InitCfg = unmarshalledConfig
			case machine.TypeControlPlane:
				bundle.ControlPlaneCfg = unmarshalledConfig
			case machine.TypeJoin:
				bundle.JoinCfg = unmarshalledConfig
			}
		}

		if err := bundle.ApplyJSONPatch(options.JSONPatch); err != nil {
			return nil, fmt.Errorf("error patching configs: %w", err)
		}

		// Pull existing talosconfig
		talosConfig, err := os.Open(filepath.Join(options.ExistingConfigs, "talosconfig"))
		if err != nil {
			return bundle, err
		}

		defer talosConfig.Close() //nolint:errcheck

		if bundle.TalosCfg, err = config.ReadFrom(talosConfig); err != nil {
			return bundle, err
		}

		return bundle, nil
	}

	// Handle generating net-new configs
	if options.Verbose {
		fmt.Println("generating PKI and tokens")
	}

	if options.InputOptions == nil {
		return nil, fmt.Errorf("no WithInputOptions is defined")
	}

	secrets, err := generate.NewSecretsBundle(generate.NewClock(), options.InputOptions.GenOptions...)
	if err != nil {
		return bundle, err
	}

	var input *generate.Input

	input, err = generate.NewInput(
		options.InputOptions.ClusterName,
		options.InputOptions.Endpoint,
		options.InputOptions.KubeVersion,
		secrets,
		options.InputOptions.GenOptions...,
	)
	if err != nil {
		return bundle, err
	}

	for _, configType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeJoin} {
		var generatedConfig *v1alpha1.Config

		generatedConfig, err = generate.Config(configType, input)
		if err != nil {
			return bundle, err
		}

		switch configType { //nolint:exhaustive
		case machine.TypeInit:
			bundle.InitCfg = generatedConfig
		case machine.TypeControlPlane:
			bundle.ControlPlaneCfg = generatedConfig
		case machine.TypeJoin:
			bundle.JoinCfg = generatedConfig
		}
	}

	if err = bundle.ApplyJSONPatch(options.JSONPatch); err != nil {
		return nil, fmt.Errorf("error patching configs: %w", err)
	}

	bundle.TalosCfg, err = generate.Talosconfig(input, options.InputOptions.GenOptions...)
	if err != nil {
		return bundle, err
	}

	return bundle, nil
}
