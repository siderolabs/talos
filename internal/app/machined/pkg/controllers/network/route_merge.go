// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NewRouteMergeController initializes a RouteMergeController.
//
// RouteMergeController merges network.RouteSpec in network.ConfigNamespace and produces final network.RouteSpec in network.Namespace.
func NewRouteMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.RouteSpec]) map[resource.ID]*network.RouteSpecSpec {
			// route is allowed as long as it's not duplicate, for duplicate higher layer takes precedence
			routes := map[string]*network.RouteSpecSpec{}

			for route := range list.All() {
				id := network.RouteID(route.TypedSpec().Table, route.TypedSpec().Family, route.TypedSpec().Destination, route.TypedSpec().Priority)

				existing, ok := routes[id]
				if ok {
					if existing.ConfigLayer > route.TypedSpec().ConfigLayer {
						// skip this route, as existing one is higher layer
						continue
					}

					if existing.ConfigLayer == route.TypedSpec().ConfigLayer {
						// same layer conflict: the kernel keeps a single route per key, so only one of them can be installed,
						// and the choice (the last one in the resource ID order) is arbitrary
						logger.Warn(
							"conflicting routes with the same key, keeping the last one",
							zap.String("route", id),
							zap.Stringer("layer", route.TypedSpec().ConfigLayer),
							zap.String("kept", route.Metadata().ID()),
							zap.String("dropped_gateway", routeSpecGatewayString(existing)),
							zap.String("dropped_link", existing.OutLinkName),
							zap.String("gateway", routeSpecGatewayString(route.TypedSpec())),
							zap.String("link", route.TypedSpec().OutLinkName),
						)
					}
				}

				routes[id] = route.TypedSpec()
			}

			return routes
		},
	)
}
