// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// HostnameConfigController manages network.HostnameSpec based on machine configuration, kernel cmdline.
type HostnameConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *HostnameConfigController) Name() string {
	return "network.HostnameConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *HostnameConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        pointer.ToString(network.NodeAddressDefaultID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *HostnameConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.HostnameSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *HostnameConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		touchedIDs := make(map[resource.ID]struct{})

		var cfgProvider talosconfig.Provider

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			cfgProvider = cfg.(*config.MachineConfig).Config()
		}

		var specs []network.HostnameSpecSpec

		// defaults
		var defaultAddr *network.NodeAddress

		addrs, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressDefaultID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			defaultAddr = addrs.(*network.NodeAddress) //nolint:errcheck,forcetypeassert
		}

		specs = append(specs, ctrl.getDefault(defaultAddr))

		// parse kernel cmdline for the default gateway
		cmdlineHostname := ctrl.parseCmdline(logger)
		if cmdlineHostname.Hostname != "" {
			specs = append(specs, cmdlineHostname)
		}

		// parse machine configuration for specs
		if cfgProvider != nil {
			configHostname := ctrl.parseMachineConfiguration(logger, cfgProvider)

			if configHostname.Hostname != "" {
				specs = append(specs, configHostname)
			}
		}

		var ids []string

		ids, err = ctrl.apply(ctx, r, specs)
		if err != nil {
			return fmt.Errorf("error applying specs: %w", err)
		}

		for _, id := range ids {
			touchedIDs[id] = struct{}{}
		}

		// list specs for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.HostnameSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				// skip specs created by other controllers
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}
	}
}

//nolint:dupl
func (ctrl *HostnameConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.HostnameSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(specs))

	for _, spec := range specs {
		spec := spec
		id := network.LayeredID(spec.ConfigLayer, network.HostnameID)

		if err := r.Modify(
			ctx,
			network.NewHostnameSpec(network.ConfigNamespaceName, id),
			func(r resource.Resource) error {
				*r.(*network.HostnameSpec).TypedSpec() = spec

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *HostnameConfigController) getDefault(defaultAddr *network.NodeAddress) (spec network.HostnameSpecSpec) {
	if defaultAddr == nil || len(defaultAddr.TypedSpec().Addresses) != 1 {
		return
	}

	spec.Hostname = fmt.Sprintf("talos-%s", strings.ReplaceAll(strings.ReplaceAll(defaultAddr.TypedSpec().Addresses[0].String(), ":", ""), ".", "-"))
	spec.ConfigLayer = network.ConfigDefault

	return spec
}

func (ctrl *HostnameConfigController) parseCmdline(logger *zap.Logger) (spec network.HostnameSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Warn("ignoring error", zap.Error(err))

		return
	}

	if settings.Hostname == "" {
		return
	}

	if err = spec.ParseFQDN(settings.Hostname); err != nil {
		logger.Warn("ignoring error", zap.Error(err))

		return network.HostnameSpecSpec{}
	}

	spec.ConfigLayer = network.ConfigCmdline

	return spec
}

func (ctrl *HostnameConfigController) parseMachineConfiguration(logger *zap.Logger, cfgProvider talosconfig.Provider) (spec network.HostnameSpecSpec) {
	hostname := cfgProvider.Machine().Network().Hostname()

	if hostname == "" {
		return
	}

	if err := spec.ParseFQDN(hostname); err != nil {
		logger.Warn("ignoring error", zap.Error(err))

		return network.HostnameSpecSpec{}
	}

	spec.ConfigLayer = network.ConfigMachineConfiguration

	return spec
}
