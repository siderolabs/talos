// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ResolverConfigController manages network.ResolverSpec based on machine configuration, kernel cmdline.
type ResolverConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *ResolverConfigController) Name() string {
	return "network.ResolverConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ResolverConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ResolverConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.ResolverSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ResolverConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		touchedIDs := make(map[resource.ID]struct{})

		var cfgProvider talosconfig.Config

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else if cfg.Config().Machine() != nil {
			cfgProvider = cfg.Config()
		}

		var specs []network.ResolverSpecSpec

		// defaults
		specs = append(specs, ctrl.getDefault())

		// parse kernel cmdline for the default gateway
		cmdlineServers := ctrl.parseCmdline(logger)
		if cmdlineServers.DNSServers != nil {
			specs = append(specs, cmdlineServers)
		}

		// parse machine configuration for specs
		if cfgProvider != nil {
			configServers := ctrl.parseMachineConfiguration(logger, cfgProvider)

			if configServers.DNSServers != nil {
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
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.ResolverSpecType, "", resource.VersionUndefined))
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

		r.ResetRestartBackoff()
	}
}

//nolint:dupl
func (ctrl *ResolverConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.ResolverSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(specs))

	for _, spec := range specs {
		id := network.LayeredID(spec.ConfigLayer, network.ResolverID)

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewResolverSpec(network.ConfigNamespaceName, id),
			func(r *network.ResolverSpec) error {
				*r.TypedSpec() = spec

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *ResolverConfigController) getDefault() (spec network.ResolverSpecSpec) {
	spec.DNSServers = []netip.Addr{netip.MustParseAddr(constants.DefaultPrimaryResolver), netip.MustParseAddr(constants.DefaultSecondaryResolver)}
	spec.ConfigLayer = network.ConfigDefault

	return spec
}

func (ctrl *ResolverConfigController) parseCmdline(logger *zap.Logger) (spec network.ResolverSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Warn("ignoring error", zap.Error(err))

		return
	}

	if len(settings.DNSAddresses) == 0 {
		return
	}

	spec.DNSServers = settings.DNSAddresses
	spec.ConfigLayer = network.ConfigCmdline

	return spec
}

func (ctrl *ResolverConfigController) parseMachineConfiguration(logger *zap.Logger, cfgProvider talosconfig.Config) (spec network.ResolverSpecSpec) {
	resolvers := cfgProvider.Machine().Network().Resolvers()

	if len(resolvers) == 0 {
		return
	}

	for i := range resolvers {
		server, err := netip.ParseAddr(resolvers[i])
		if err != nil {
			logger.Warn("failed to parse DNS server", zap.String("server", resolvers[i]), zap.Error(err))

			continue
		}

		spec.DNSServers = append(spec.DNSServers, server)
	}

	spec.ConfigLayer = network.ConfigMachineConfiguration

	return spec
}
