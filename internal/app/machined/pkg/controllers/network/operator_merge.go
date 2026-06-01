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

// NewOperatorMergeController initializes a OperatorMergeController.
//
// OperatorMergeController merges network.OperatorSpec in network.ConfigNamespace and produces final network.OperatorSpec in network.Namespace.
func NewOperatorMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.OperatorSpec]) map[resource.ID]*network.OperatorSpecSpec {
			// operator is allowed as long as it's not duplicate, for duplicate higher layer takes precedence
			operators := map[string]*network.OperatorSpecSpec{}

			for operator := range list.All() {
				id := network.OperatorID(*operator.TypedSpec())

				existing, ok := operators[id]
				if ok && existing.ConfigLayer > operator.TypedSpec().ConfigLayer {
					// skip this operator, as existing one is higher layer
					continue
				}

				operators[id] = operator.TypedSpec()
			}

			return operators
		},
	)
}
