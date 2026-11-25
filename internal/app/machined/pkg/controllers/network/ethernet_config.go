// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	networkpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/network"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configtypes "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EthernetConfigController manages network.EthernetSpec based on machine configuration.
type EthernetConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *EthernetConfigController) Name() string {
	return "network.EthernetConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EthernetConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EthernetConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.EthernetSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *EthernetConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error reading machine configuration: %w", err)
		}

		linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing link statuses: %w", err)
		}

		if cfg != nil {
			if err = ctrl.apply(ctx, r, logger, cfg.Config().EthernetConfigs(), linkStatuses); err != nil {
				return fmt.Errorf("error applying EthernetSpec: %w", err)
			}
		}

		if err = r.CleanupOutputs(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.EthernetSpecType, "", resource.VersionUndefined)); err != nil {
			return fmt.Errorf("error cleaning up EthernetSpec: %w", err)
		}
	}
}

func (ctrl *EthernetConfigController) apply(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	configs []configtypes.EthernetConfig,
	linkStatuses safe.List[*network.LinkStatus],
) error {
	linkNameResolver := network.NewLinkResolver(linkStatuses.All)

	// built lazily, as it's only needed by configs with a selector
	var physicalLinks map[string]*networkpb.LinkStatusSpec

	for _, ethCfg := range configs {
		if ethCfg.LinkSelector().IsZero() {
			if err := writeEthernetSpec(ctx, r, ethCfg.Name(), linkNameResolver.Resolve(ethCfg.Name()), false, ethCfg); err != nil {
				return err
			}

			continue
		}

		if physicalLinks == nil {
			var err error

			if physicalLinks, err = getPhysicalLinks(linkStatuses); err != nil {
				return err
			}
		}

		for name, link := range physicalLinks {
			matches, err := ethCfg.LinkSelector().EvalBool(celenv.LinkLocator(), map[string]any{
				"link": link,
			})
			if err != nil {
				// don't fail the whole reconcile, as it would block other configs and the cleanup
				logger.Error("error evaluating link selector, skipping link",
					zap.String("config", ethCfg.Name()),
					zap.String("link", name),
					zap.Error(err),
				)

				continue
			}

			if !matches {
				continue
			}

			// specs matched by a selector are IDed "<config-name>/<link-name>"
			if err := writeEthernetSpec(ctx, r, ethCfg.Name()+"/"+name, name, true, ethCfg); err != nil {
				return err
			}
		}
	}

	return nil
}

func getPhysicalLinks(linkStatuses safe.List[*network.LinkStatus]) (map[string]*networkpb.LinkStatusSpec, error) {
	physicalLinks := map[string]*networkpb.LinkStatusSpec{}

	for link := range linkStatuses.All() {
		if !link.TypedSpec().Physical() {
			continue
		}

		var spec networkpb.LinkStatusSpec

		if err := proto.ResourceSpecToProto(link, &spec); err != nil {
			return nil, fmt.Errorf("error converting link spec (%s) to proto: %w", link.Metadata().ID(), err)
		}

		physicalLinks[link.Metadata().ID()] = &spec
	}

	return physicalLinks, nil
}

func writeEthernetSpec(
	ctx context.Context,
	r controller.Runtime,
	id, linkName string,
	fromSelector bool,
	cfg configtypes.EthernetConfig,
) error {
	if err := safe.WriterModify(ctx, r, network.NewEthernetSpec(network.ConfigNamespaceName, id), func(spec *network.EthernetSpec) error {
		spec.TypedSpec().Rings = network.EthernetRingsSpec(cfg.Rings())
		spec.TypedSpec().Channels = network.EthernetChannelsSpec(cfg.Channels())
		spec.TypedSpec().Features = cfg.Features()
		spec.TypedSpec().WakeOnLAN = cfg.WakeOnLAN()
		spec.TypedSpec().FlowControl = network.EthernetFlowControlSpec(cfg.FlowControl())
		spec.TypedSpec().LinkName = linkName
		spec.TypedSpec().FromSelector = fromSelector

		return nil
	}); err != nil {
		return fmt.Errorf("error writing EthernetSpec %q: %w", id, err)
	}

	return nil
}
