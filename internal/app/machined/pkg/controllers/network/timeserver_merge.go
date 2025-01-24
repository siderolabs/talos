// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
package network

import (
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NewTimeServerMergeController initializes a TimeServerMergeController.
//
// TimeServerMergeController merges network.TimeServerSpec in network.ConfigNamespace and produces final network.TimeServerSpec in network.Namespace.
func NewTimeServerMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.TimeServerSpec]) map[resource.ID]*network.TimeServerSpecSpec {
			// simply merge by layers, overriding with the next configuration layer
			var final network.TimeServerSpecSpec

			for spec := range list.All() {
				if final.NTPServers != nil && spec.TypedSpec().ConfigLayer < final.ConfigLayer {
					// skip this spec, as existing one is higher layer
					continue
				}

				if spec.TypedSpec().ConfigLayer == final.ConfigLayer {
					// merge server lists on the same level
					final.NTPServers = append(final.NTPServers, spec.TypedSpec().NTPServers...)
				} else {
					// otherwise, replace the lists
					final = *spec.TypedSpec()
				}
			}

			if final.NTPServers != nil {
				return map[resource.ID]*network.TimeServerSpecSpec{
					network.TimeServerID: &final,
				}
			}

			return nil
		},
	)
}
