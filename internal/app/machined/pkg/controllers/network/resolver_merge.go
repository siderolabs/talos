// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
//
//nolint:dupl
package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ResolverMergeController merges network.ResolverSpec in network.ConfigNamespace and produces final network.ResolverSpec in network.Namespace.
type ResolverMergeController struct{}

// Name implements controller.Controller interface.
func (ctrl *ResolverMergeController) Name() string {
	return "network.ResolverMergeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ResolverMergeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.ConfigNamespaceName,
			Type:      network.ResolverSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.ResolverSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ResolverMergeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.ResolverSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ResolverMergeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.ResolverSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// simply merge by layers, overriding with the next configuration layer
		var final network.ResolverSpecSpec

		for _, res := range list.Items {
			spec := res.(*network.ResolverSpec) //nolint:errcheck,forcetypeassert

			if final.DNSServers != nil && spec.TypedSpec().ConfigLayer < final.ConfigLayer {
				// skip this spec, as existing one is higher layer
				continue
			}

			if spec.TypedSpec().ConfigLayer == final.ConfigLayer {
				// merge server lists on the same level
				final.DNSServers = append(final.DNSServers, spec.TypedSpec().DNSServers...)
			} else {
				// otherwise, replace the lists
				final = *spec.TypedSpec()
			}
		}

		if final.DNSServers != nil {
			if err = r.Modify(ctx, network.NewResolverSpec(network.NamespaceName, network.ResolverID), func(res resource.Resource) error {
				spec := res.(*network.ResolverSpec) //nolint:errcheck,forcetypeassert

				*spec.TypedSpec() = final

				return nil
			}); err != nil {
				if state.IsPhaseConflictError(err) {
					// conflict
					final.DNSServers = nil

					r.QueueReconcile()
				} else {
					return fmt.Errorf("error updating resource: %w", err)
				}
			}
		}

		if final.DNSServers == nil {
			// remove existing
			var okToDestroy bool

			md := resource.NewMetadata(network.NamespaceName, network.ResolverSpecType, network.ResolverID, resource.VersionUndefined)

			okToDestroy, err = r.Teardown(ctx, md)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error cleaning up specs: %w", err)
			}

			if okToDestroy {
				if err = r.Destroy(ctx, md); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}
	}
}
