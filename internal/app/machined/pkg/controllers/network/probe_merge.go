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

// NewProbeMergeController initializes a ProbeMergeController.
//
// ProbeMergeController merges network.ProbeSpec in network.ConfigNamespace and produces final network.ProbeSpec in network.Namespace.
func NewProbeMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.ProbeSpec]) map[resource.ID]*network.ProbeSpecSpec {
			// sort by link name, configuration layer
			list.SortFunc(func(left, right *network.ProbeSpec) int {
				return cmp.Compare(left.TypedSpec().ConfigLayer, right.TypedSpec().ConfigLayer)
			})

			// build final probe definition merging multiple layers
			probes := make(map[string]*network.ProbeSpecSpec, list.Len())

			for probe := range list.All() {
				id, err := probe.TypedSpec().ID()
				if err != nil {
					logger.Warn("error getting probe ID", zap.Error(err))

					continue
				}

				// no way to actually have multiple probes with the same ID in different layers,
				// so we can just merge them one by one
				probes[id] = probe.TypedSpec()
			}

			return probes
		},
	)
}
