// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
package network

import (
	"cmp"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NewResolverMergeController initializes a ResolverMergeController.
//
// ResolverMergeController merges network.ResolverSpec in network.ConfigNamespace and produces final network.ResolverSpec in network.Namespace.
func NewResolverMergeController() controller.Controller {
	return GenericMergeController(
		network.ConfigNamespaceName,
		network.NamespaceName,
		func(logger *zap.Logger, list safe.List[*network.ResolverSpec]) map[resource.ID]*network.ResolverSpecSpec {
			// sort by config layer
			list.SortFunc(func(l, r *network.ResolverSpec) int {
				return cmp.Compare(l.TypedSpec().ConfigLayer, r.TypedSpec().ConfigLayer)
			})

			// simply merge by layers, overriding with the next configuration layer
			var final network.ResolverSpecSpec

			for res := range list.All() {
				spec := res.TypedSpec()

				domainPos := 0

				for _, domain := range spec.SearchDomains {
					if !slices.Contains(final.SearchDomains, domain) {
						final.SearchDomains = slices.Insert(final.SearchDomains, domainPos, domain)
						domainPos++
					}
				}

				switch spec.ConfigLayer { //nolint:exhaustive
				case final.ConfigLayer:
					// simply append server lists on the same layer
					final.NameServers = append(final.NameServers, spec.NameServers...)
				case network.ConfigMachineConfiguration:
					// machine configuration overrides previous layers, but only when DNS servers are set
					if len(spec.NameServers) > 0 {
						final.NameServers = slices.Clone(spec.NameServers)
					}
				default:
					// otherwise, do a smart merge across IPv4/IPv6
					mergeNameServers(&final.NameServers, spec.NameServers)
				}

				final.ConfigLayer = spec.ConfigLayer
			}

			if final.NameServers != nil {
				final.Convert() // convert deprecated fields for backward compatibility

				return map[resource.ID]*network.ResolverSpecSpec{
					network.ResolverID: &final,
				}
			}

			return nil
		},
	)
}

func mergeNameServers(dst *[]network.NameServerSpec, src []network.NameServerSpec) {
	if *dst == nil {
		*dst = slices.Clone(src)

		return
	}

	srcHasV4 := slices.IndexFunc(src, func(ns network.NameServerSpec) bool { return ns.Addr.Is4() }) != -1
	srcHasV6 := slices.IndexFunc(src, func(ns network.NameServerSpec) bool { return ns.Addr.Is6() }) != -1
	dstHasV4 := slices.IndexFunc(*dst, func(ns network.NameServerSpec) bool { return ns.Addr.Is4() }) != -1
	dstHasV6 := slices.IndexFunc(*dst, func(ns network.NameServerSpec) bool { return ns.Addr.Is6() }) != -1

	// if old set has IPv4, and new one doesn't, preserve IPv4
	// and same vice versa for IPv6
	switch {
	case dstHasV4 && !srcHasV4:
		*dst = slices.Concat(src, xslices.Filter(*dst, func(ns network.NameServerSpec) bool { return ns.Addr.Is4() }))
	case dstHasV6 && !srcHasV6:
		*dst = slices.Concat(src, xslices.Filter(*dst, func(ns network.NameServerSpec) bool { return ns.Addr.Is6() }))
	default:
		*dst = slices.Clone(src)
	}
}
