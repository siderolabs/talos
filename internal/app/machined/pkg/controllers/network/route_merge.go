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
				id := network.RouteID(route.TypedSpec().Table, route.TypedSpec().Family, route.TypedSpec().Destination, route.TypedSpec().Gateway, route.TypedSpec().Priority, route.TypedSpec().OutLinkName)

				existing, ok := routes[id]
				if ok && existing.ConfigLayer > route.TypedSpec().ConfigLayer {
					// skip this route, as existing one is higher layer
					continue
				}

				routes[id] = route.TypedSpec()
			}

			return routes
		},
	)
}
