// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"cmp"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NewEthernetMergeController initializes a EthernetSpecMergeController.
//
// EthernetMergeController merges network.EthernetSpec in network.ConfigNamespace and produces final network.EthernetSpec in network.Namespace.
// EthernetSpecs matched by a selector are merged first in no particular order,
// then EthernetSpecs matched by link alias, and EthernetSpecs matched by exact link name are merged last,
// so each group overrides the previous ones.
func NewEthernetMergeController() controller.Controller {
	return GenericMergeController(network.ConfigNamespaceName, network.NamespaceName, mergeEthernetSpecs)
}

// ethernetSpecPriority returns the merge priority of the spec, specs with a higher priority override the lower ones.
func ethernetSpecPriority(spec *network.EthernetSpec) int {
	switch {
	case spec.TypedSpec().FromSelector:
		return 0
	case spec.Metadata().ID() != spec.TypedSpec().LinkName: // matched by alias
		return 1
	default: // matched by exact link name
		return 2
	}
}

func mergeEthernetSpecs(_ *zap.Logger, list safe.List[*network.EthernetSpec]) map[resource.ID]*network.EthernetSpecSpec {
	specs := make(map[resource.ID]*network.EthernetSpecSpec, list.Len())

	sorted := slices.SortedStableFunc(list.All(), func(a, b *network.EthernetSpec) int {
		return cmp.Compare(ethernetSpecPriority(a), ethernetSpecPriority(b))
	})

	for _, spec := range sorted {
		linkName := spec.TypedSpec().LinkName

		if existing, ok := specs[linkName]; ok {
			existing.Merge(spec.TypedSpec())
		} else {
			merged := spec.TypedSpec().DeepCopy()
			merged.FromSelector = false // the merged spec may combine several sources

			specs[linkName] = &merged
		}
	}

	return specs
}
