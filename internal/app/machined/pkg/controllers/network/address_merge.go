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

// NewAddressMergeController initializes a AddressMergeController.
//
// AddressMergeController merges network.AddressSpec in network.ConfigNamespace and produces final network.AddressSpec in network.Namespace.
func NewAddressMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.AddressSpec]) map[resource.ID]*network.AddressSpecSpec {
			// address is allowed as long as it's not duplicate, for duplicate higher layer takes precedence
			addresses := map[resource.ID]*network.AddressSpecSpec{}

			for address := range list.All() {
				id := network.AddressID(address.TypedSpec().LinkName, address.TypedSpec().Address)

				existing, ok := addresses[id]
				if ok && existing.ConfigLayer > address.TypedSpec().ConfigLayer {
					// skip this address, as existing one is higher layer
					continue
				}

				addresses[id] = address.TypedSpec()
			}

			return addresses
		},
	)
}
