// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// LinkAliasSpecController applies network.LinkAliasSpec to the actual interfaces.
type LinkAliasSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *LinkAliasSpecController) Name() string {
	return "network.LinkAliasSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LinkAliasSpecController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *LinkAliasSpecController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LinkAliasSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// wait for udevd to be healthy, which implies that all link renames are done
	if err := runtime.WaitForDevicesReady(ctx, r,
		[]controller.Input{
			{
				Namespace: network.NamespaceName,
				Type:      network.LinkAliasSpecType,
				Kind:      controller.InputWeak,
			},
		},
	); err != nil {
		return err
	}

	// watch link changes as some routes might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(watch.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_LINK)
	if err != nil {
		return err
	}

	defer watcher.Done()

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source link alias specs (what should be aliased)
		linkAliasSpecs, err := safe.ReaderListAll[*network.LinkAliasSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing link alias specs: %w", err)
		}

		linkAliasSpecLookup := make(map[string]string, linkAliasSpecs.Len())

		for linkAliasSpec := range linkAliasSpecs.All() {
			linkAliasSpecLookup[linkAliasSpec.Metadata().ID()] = linkAliasSpec.TypedSpec().Alias
		}

		logger.Debug("reconciling link aliases", zap.Any("desired", linkAliasSpecLookup))

		// list rtnetlink links (interfaces)
		links, err := conn.Link.List()
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		// loop over links and make reconcile decision
		var multiErr *multierror.Error

		for _, link := range links {
			if link.Attributes == nil {
				continue
			}

			if link.Attributes.Info != nil || nethelpers.LinkType(link.Type) != nethelpers.LinkEther {
				// skip non-physical links
				continue
			}

			expectedAlias, shouldHaveAlias := linkAliasSpecLookup[link.Attributes.Name]
			currentAlias := pointer.SafeDeref(link.Attributes.Alias)

			if !shouldHaveAlias && currentAlias != "" {
				// should not have alias, but has one - remove it
				logger.Info("removing link alias",
					zap.String("link", link.Attributes.Name),
					zap.String("alias", currentAlias),
				)

				if err = conn.Link.Set(&rtnetlink.LinkMessage{
					Index: link.Index,
					Attributes: &rtnetlink.LinkAttributes{
						Alias: new(""),
					},
				}); err != nil {
					multiErr = multierror.Append(multiErr, fmt.Errorf("error removing alias %q from link %q: %w", currentAlias, link.Attributes.Name, err))
				}
			} else if shouldHaveAlias && currentAlias != expectedAlias {
				// should have alias, but doesn't have it or it's different - set it
				logger.Info("setting link alias",
					zap.String("link", link.Attributes.Name),
					zap.String("alias", expectedAlias),
				)

				if err = conn.Link.Set(&rtnetlink.LinkMessage{
					Index: link.Index,
					Attributes: &rtnetlink.LinkAttributes{
						Alias: new(expectedAlias),
					},
				}); err != nil {
					multiErr = multierror.Append(multiErr, fmt.Errorf("error setting alias %q on link %q: %w", expectedAlias, link.Attributes.Name, err))
				}
			}
		}

		if err = multiErr.ErrorOrNil(); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}
