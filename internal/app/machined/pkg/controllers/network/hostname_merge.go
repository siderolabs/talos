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

// NewHostnameMergeController initializes a HostnameMergeController.
//
// HostnameMergeController merges network.HostnameSpec in network.ConfigNamespace and produces final network.HostnameSpec in network.Namespace.
func NewHostnameMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.HostnameSpec]) map[resource.ID]*network.HostnameSpecSpec {
			// simply merge by layers, overriding with the next configuration layer
			var final network.HostnameSpecSpec

			for spec := range list.All() {
				if final.Hostname != "" && spec.TypedSpec().ConfigLayer <= final.ConfigLayer {
					// skip this spec, as existing one is higher layer
					continue
				}

				final = *spec.TypedSpec()
			}

			if final.Hostname != "" {
				return map[resource.ID]*network.HostnameSpecSpec{
					network.HostnameID: &final,
				}
			}

			return nil
		},
	)
}
