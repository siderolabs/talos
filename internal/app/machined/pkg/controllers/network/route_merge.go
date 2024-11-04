// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// RouteMergeController merges network.RouteSpec in network.ConfigNamespace and produces final network.RouteSpec in network.Namespace.
type RouteMergeController struct{}

// Name implements controller.Controller interface.
func (ctrl *RouteMergeController) Name() string {
	return "network.RouteMergeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RouteMergeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.ConfigNamespaceName,
			Type:      network.RouteSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.RouteSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RouteMergeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.RouteSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RouteMergeController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.RouteSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network routes: %w", err)
		}

		// route is allowed as long as it's not duplicate, for duplicate higher layer takes precedence
		routes := map[string]*network.RouteSpec{}

		for _, res := range list.Items {
			route := res.(*network.RouteSpec) //nolint:errcheck,forcetypeassert
			id := network.RouteID(route.TypedSpec().Table, route.TypedSpec().Family, route.TypedSpec().Destination, route.TypedSpec().Gateway, route.TypedSpec().Priority, route.TypedSpec().OutLinkName)

			existing, ok := routes[id]
			if ok && existing.TypedSpec().ConfigLayer > route.TypedSpec().ConfigLayer {
				// skip this route, as existing one is higher layer
				continue
			}

			routes[id] = route
		}

		conflictsDetected := 0

		for id, route := range routes {
			if err = safe.WriterModify(ctx, r, network.NewRouteSpec(network.NamespaceName, id), func(rt *network.RouteSpec) error {
				*rt.TypedSpec() = *route.TypedSpec()

				return nil
			}); err != nil {
				if state.IsPhaseConflictError(err) {
					// phase conflict, resource is being torn down, skip updating it and trigger reconcile
					// later by failing the
					conflictsDetected++

					delete(routes, id)
				} else {
					return fmt.Errorf("error updating resource: %w", err)
				}
			}
		}

		// list routes for cleanup
		list, err = r.List(ctx, resource.NewMetadata(network.NamespaceName, network.RouteSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if _, ok := routes[res.Metadata().ID()]; !ok {
				var okToDestroy bool

				okToDestroy, err = r.Teardown(ctx, res.Metadata())
				if err != nil {
					return fmt.Errorf("error cleaning up routes: %w", err)
				}

				if okToDestroy {
					if err = r.Destroy(ctx, res.Metadata()); err != nil {
						return fmt.Errorf("error cleaning up routes: %w", err)
					}
				}
			}
		}

		if conflictsDetected > 0 {
			return fmt.Errorf("%d conflict(s) detected", conflictsDetected)
		}

		r.ResetRestartBackoff()
	}
}
