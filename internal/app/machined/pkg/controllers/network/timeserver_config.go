// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// TimeServerConfigController manages network.TimeServerSpec based on machine configuration, kernel cmdline.
type TimeServerConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *TimeServerConfigController) Name() string {
	return "network.TimeServerConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *TimeServerConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *TimeServerConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.TimeServerSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *TimeServerConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

		var specs []network.TimeServerSpecSpec

		// defaults
		specs = append(specs, ctrl.getDefault())

		// parse kernel cmdline for the default gateway
		cmdlineServers := ctrl.parseCmdline(logger)
		if cmdlineServers.NTPServers != nil {
			specs = append(specs, cmdlineServers)
		}

		// parse machine configuration for specs
		if cfgProvider != nil {
			configServers := ctrl.parseMachineConfiguration(cfgProvider)

			if configServers.NTPServers != nil {
				specs = append(specs, configServers)
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
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.TimeServerSpecType, "", resource.VersionUndefined))
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
func (ctrl *TimeServerConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.TimeServerSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(specs))

	for _, spec := range specs {
		spec := spec
		id := network.LayeredID(spec.ConfigLayer, network.TimeServerID)

		if err := r.Modify(
			ctx,
			network.NewTimeServerSpec(network.ConfigNamespaceName, id),
			func(r resource.Resource) error {
				*r.(*network.TimeServerSpec).TypedSpec() = spec

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *TimeServerConfigController) getDefault() (spec network.TimeServerSpecSpec) {
	spec.NTPServers = []string{constants.DefaultNTPServer}
	spec.ConfigLayer = network.ConfigDefault

	return spec
}

func (ctrl *TimeServerConfigController) parseCmdline(logger *zap.Logger) (spec network.TimeServerSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Warn("ignoring error", zap.Error(err))

		return
	}

	if len(settings.NTPAddresses) == 0 {
		return
	}

	spec.NTPServers = make([]string, len(settings.NTPAddresses))
	spec.ConfigLayer = network.ConfigCmdline

	for i := range settings.NTPAddresses {
		spec.NTPServers[i] = settings.NTPAddresses[i].String()
	}

	return spec
}

func (ctrl *TimeServerConfigController) parseMachineConfiguration(cfgProvider talosconfig.Provider) (spec network.TimeServerSpecSpec) {
	if len(cfgProvider.Machine().Time().Servers()) == 0 {
		return
	}

	spec.NTPServers = append([]string(nil), cfgProvider.Machine().Time().Servers()...)
	spec.ConfigLayer = network.ConfigMachineConfiguration

	return spec
}
