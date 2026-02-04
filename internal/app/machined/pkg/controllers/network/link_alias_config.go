// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	networkpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/network"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// LinkAliasConfigController manages network.LinkAliasSpec based on machine configuration, list of links, etc.
type LinkAliasConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *LinkAliasConfigController) Name() string {
	return "network.LinkAliasConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LinkAliasConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LinkAliasConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.LinkAliasSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LinkAliasConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting machine config: %w", err)
			}
		}

		linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing link statuses: %w", err)
		}

		// we are only interested in physical links
		physicalLinks := xslices.Filter(slices.Collect(linkStatuses.All()), func(item *network.LinkStatus) bool {
			return item.TypedSpec().Physical()
		})

		physicalLinkSpecs := make([]*networkpb.LinkStatusSpec, 0, len(physicalLinks))

		for _, link := range physicalLinks {
			var spec networkpb.LinkStatusSpec

			if err = proto.ResourceSpecToProto(link, &spec); err != nil {
				return fmt.Errorf("error converting link spec (%s) to proto: %w", link.Metadata().ID(), err)
			}

			physicalLinkSpecs = append(physicalLinkSpecs, &spec)
		}

		var linkAliasConfigs []configconfig.NetworkLinkAliasConfig

		if cfg != nil {
			linkAliasConfigs = cfg.Config().NetworkLinkAliasConfigs()
		}

		linkAliases := map[string]string{}

		for _, lac := range linkAliasConfigs {
			var matchedLinks []*network.LinkStatus

			for idx, link := range physicalLinkSpecs {
				// Skip links that already have an alias if skipAliasedLinks is enabled
				if lac.SkipAliasedLinks() {
					if _, ok := linkAliases[physicalLinks[idx].Metadata().ID()]; ok {
						continue
					}
				}

				matches, err := lac.LinkSelector().EvalBool(celenv.LinkLocator(), map[string]any{
					"link": link,
				})
				if err != nil {
					return fmt.Errorf("error evaluating link selector: %w", err)
				}

				if matches {
					matchedLinks = append(matchedLinks, physicalLinks[idx])
				}
			}

			if len(matchedLinks) == 0 {
				continue
			}

			if len(matchedLinks) > 1 {
				if lac.RequireUniqueMatch() {
					logger.Warn("link selector matched multiple links, skipping",
						zap.String("selector", lac.LinkSelector().String()),
						zap.String("alias", lac.Name()),
						zap.Strings("links", xslices.Map(matchedLinks, func(item *network.LinkStatus) string {
							return item.Metadata().ID()
						})),
					)

					continue
				}

				logger.Info("link selector matched multiple links, using first match",
					zap.String("selector", lac.LinkSelector().String()),
					zap.String("alias", lac.Name()),
					zap.String("selected_link", matchedLinks[0].Metadata().ID()),
					zap.Strings("links", xslices.Map(matchedLinks, func(item *network.LinkStatus) string {
						return item.Metadata().ID()
					})),
				)
			}

			matchedLink := matchedLinks[0]

			if _, ok := linkAliases[matchedLink.Metadata().ID()]; ok {
				logger.Warn("link already has an alias, skipping",
					zap.String("link", matchedLink.Metadata().ID()),
					zap.String("existing_alias", linkAliases[matchedLink.Metadata().ID()]),
					zap.String("new_alias", lac.Name()),
				)

				continue
			}

			linkAliases[matchedLink.Metadata().ID()] = lac.Name()
		}

		for linkID, alias := range linkAliases {
			if err = safe.WriterModify(
				ctx,
				r,
				network.NewLinkAliasSpec(network.NamespaceName, linkID),
				func(r *network.LinkAliasSpec) error {
					r.TypedSpec().Alias = alias

					return nil
				},
			); err != nil {
				return fmt.Errorf("error writing link alias spec for link %q: %w", linkID, err)
			}
		}

		if err := safe.CleanupOutputs[*network.LinkAliasSpec](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up link alias specs: %w", err)
		}
	}
}
