// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bundle provides a set of machine configuration files.
package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// Bundle defines a set of machine configuration files.
type Bundle struct {
	InitCfg         config.Provider
	ControlPlaneCfg config.Provider
	WorkerCfg       config.Provider
	TalosCfg        *clientconfig.Config
}

// NewBundle returns a new bundle of configuration files.
//
//nolint:gocyclo,cyclop
func NewBundle(opts ...Option) (*Bundle, error) {
	options := DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	bundle := &Bundle{}

	// Configs already exist, we'll pull them in.
	if options.ExistingConfigs != "" {
		if options.InputOptions != nil {
			return bundle, errors.New("both existing config path and input options specified")
		}

		// Pull existing machine configs of each type
		for _, configType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
			data, err := os.ReadFile(filepath.Join(options.ExistingConfigs, strings.ToLower(configType.String())+".yaml"))
			if err != nil {
				if configType == machine.TypeInit && os.IsNotExist(err) {
					continue
				}

				return bundle, err
			}

			unmarshalledConfig, err := configloader.NewFromBytes(data)
			if err != nil {
				return nil, err
			}

			switch configType {
			case machine.TypeInit:
				bundle.InitCfg = unmarshalledConfig
			case machine.TypeControlPlane:
				bundle.ControlPlaneCfg = unmarshalledConfig
			case machine.TypeWorker:
				bundle.WorkerCfg = unmarshalledConfig
			case machine.TypeUnknown:
				fallthrough
			default:
				panic("unreachable")
			}
		}

		if err := bundle.applyPatches(options); err != nil {
			return nil, err
		}

		// Pull existing talosconfig
		talosConfig, err := os.Open(filepath.Join(options.ExistingConfigs, "talosconfig"))
		if err != nil {
			return bundle, err
		}

		defer talosConfig.Close() //nolint:errcheck

		if bundle.TalosCfg, err = clientconfig.ReadFrom(talosConfig); err != nil {
			return bundle, err
		}

		return bundle, nil
	}

	// Handle generating net-new configs
	if options.Verbose {
		fmt.Fprintln(os.Stderr, "generating PKI and tokens")
	}

	if options.InputOptions == nil {
		return nil, errors.New("no WithInputOptions is defined")
	}

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
		var generatedConfig config.Provider

		generatedConfig, err = input.Config(configType)
		if err != nil {
			return bundle, err
		}

		switch configType {
		case machine.TypeInit:
			bundle.InitCfg = generatedConfig
		case machine.TypeControlPlane:
			bundle.ControlPlaneCfg = generatedConfig
		case machine.TypeWorker:
			bundle.WorkerCfg = generatedConfig
		case machine.TypeUnknown:
			fallthrough
		default:
			panic("unreachable")
		}
	}

	if err = bundle.applyPatches(options); err != nil {
		return nil, err
	}

	bundle.TalosCfg, err = input.Talosconfig()
	if err != nil {
		return bundle, err
	}

	return bundle, nil
}

// Init implements the ProviderBundle interface.
func (bundle *Bundle) Init() config.Provider {
	return bundle.InitCfg
}

// ControlPlane implements the ProviderBundle interface.
func (bundle *Bundle) ControlPlane() config.Provider {
	return bundle.ControlPlaneCfg
}

// Worker implements the ProviderBundle interface.
func (bundle *Bundle) Worker() config.Provider {
	return bundle.WorkerCfg
}

// TalosConfig implements the ProviderBundle interface.
func (bundle *Bundle) TalosConfig() *clientconfig.Config {
	return bundle.TalosCfg
}

// Write config files to output directory.
func (bundle *Bundle) Write(outputDir string, commentsFlags encoder.CommentsFlags, types ...machine.Type) error {
	for _, t := range types {
		name := strings.ToLower(t.String()) + ".yaml"
		fullFilePath := filepath.Join(outputDir, name)

		bytes, err := bundle.Serialize(commentsFlags, t)
		if err != nil {
			return err
		}

		if err = os.WriteFile(fullFilePath, bytes, 0o644); err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "created %s\n", fullFilePath)
	}

	return nil
}

// Serialize returns the config for the provided machine type as bytes.
func (bundle *Bundle) Serialize(commentsFlags encoder.CommentsFlags, machineType machine.Type) ([]byte, error) {
	switch machineType {
	case machine.TypeInit:
		return bundle.Init().EncodeBytes(encoder.WithComments(commentsFlags))
	case machine.TypeControlPlane:
		return bundle.ControlPlane().EncodeBytes(encoder.WithComments(commentsFlags))
	case machine.TypeWorker:
		return bundle.Worker().EncodeBytes(encoder.WithComments(commentsFlags))
	case machine.TypeUnknown:
		fallthrough
	default:
		return nil, fmt.Errorf("unexpected machine type %v", machineType)
	}
}

// ApplyPatches patches every config type with a patch.
func (bundle *Bundle) ApplyPatches(patches []configpatcher.Patch, patchControlPlane, patchWorker bool) error {
	if len(patches) == 0 {
		return nil
	}

	apply := func(in config.Provider) (config.Provider, error) {
		patched, err := configpatcher.Apply(configpatcher.WithConfig(in), patches)
		if err != nil {
			return nil, err
		}

		return patched.Config()
	}

	var err error

	if patchControlPlane {
		bundle.InitCfg, err = apply(bundle.InitCfg)
		if err != nil {
			return err
		}

		bundle.ControlPlaneCfg, err = apply(bundle.ControlPlaneCfg)
		if err != nil {
			return err
		}
	}

	if patchWorker {
		bundle.WorkerCfg, err = apply(bundle.WorkerCfg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (bundle *Bundle) applyPatches(options Options) error {
	if err := bundle.ApplyPatches(options.Patches, true, true); err != nil {
		return fmt.Errorf("error patching configs: %w", err)
	}

	if err := bundle.ApplyPatches(options.PatchesControlPlane, true, false); err != nil {
		return fmt.Errorf("error patching control plane configs: %w", err)
	}

	if err := bundle.ApplyPatches(options.PatchesWorker, false, true); err != nil {
		return fmt.Errorf("error patching worker config: %w", err)
	}

	return nil
}
