// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
package network

import (
	"cmp"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NewLinkMergeController initializes a LinkMergeController.
//
// LinkMergeController merges network.LinkSpec in network.ConfigNamespace and produces final network.AddressSpec in network.Namespace.
func NewLinkMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.LinkSpec]) map[resource.ID]*network.LinkSpecSpec {
			// sort by link name, configuration layer
			list.SortFunc(func(left, right *network.LinkSpec) int {
				if res := cmp.Compare(left.TypedSpec().Name, right.TypedSpec().Name); res != 0 {
					return res
				}

				return cmp.Compare(left.TypedSpec().ConfigLayer, right.TypedSpec().ConfigLayer)
			})

			// build final link definition merging multiple layers
			links := make(map[string]*network.LinkSpecSpec, list.Len())

			for link := range list.All() {
				id := network.LinkID(link.TypedSpec().Name)

				existing, ok := links[id]
				if !ok {
					links[id] = link.TypedSpec()
				} else if err := existing.Merge(link.TypedSpec()); err != nil {
					logger.Warn("error merging links", zap.Error(err))
				}
			}

			return links
		},
	)
}
