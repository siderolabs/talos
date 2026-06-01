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

// NewRoutingRuleMergeController initializes a RoutingRuleMergeController.
//
// RoutingRuleMergeController merges network.RoutingRuleSpec in network.ConfigNamespace and produces final network.RoutingRuleSpec in network.Namespace.
func NewRoutingRuleMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.RoutingRuleSpec]) map[resource.ID]*network.RoutingRuleSpecSpec {
			// routing rule is allowed as long as it's not duplicate, for duplicate higher layer takes precedence
			rules := map[string]*network.RoutingRuleSpecSpec{}

			for rule := range list.All() {
				id := network.RoutingRuleID(rule.TypedSpec().Family, rule.TypedSpec().Priority)

				existing, ok := rules[id]
				if ok && existing.ConfigLayer > rule.TypedSpec().ConfigLayer {
					// skip this rule, as existing one is higher layer
					continue
				}

				rules[id] = rule.TypedSpec()
			}

			return rules
		},
	)
}
