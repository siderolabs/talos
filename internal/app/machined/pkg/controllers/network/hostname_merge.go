// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides controllers which manage network resources.
package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// HostnameMergeController merges network.HostnameSpec in network.ConfigNamespace and produces final network.HostnameSpec in network.Namespace.
type HostnameMergeController struct{}

// Name implements controller.Controller interface.
func (ctrl *HostnameMergeController) Name() string {
	return "network.HostnameMergeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *HostnameMergeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.ConfigNamespaceName,
			Type:      network.HostnameSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *HostnameMergeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.HostnameSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *HostnameMergeController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.HostnameSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// simply merge by layers, overriding with the next configuration layer
		var final network.HostnameSpecSpec

		for _, res := range list.Items {
			spec := res.(*network.HostnameSpec) //nolint:errcheck,forcetypeassert

			if final.Hostname != "" && spec.TypedSpec().ConfigLayer <= final.ConfigLayer {
				// skip this spec, as existing one is higher layer
				continue
			}

			final = *spec.TypedSpec()
		}

		if final.Hostname != "" {
			if err = safe.WriterModify(ctx, r, network.NewHostnameSpec(network.NamespaceName, network.HostnameID), func(spec *network.HostnameSpec) error {
				*spec.TypedSpec() = final

				return nil
			}); err != nil {
				if state.IsPhaseConflictError(err) {
					// conflict
					final.Hostname = ""

					r.QueueReconcile()
				} else {
					return fmt.Errorf("error updating resource: %w", err)
				}
			}
		}

		if final.Hostname == "" {
			// remove existing
			var okToDestroy bool

			md := resource.NewMetadata(network.NamespaceName, network.HostnameSpecType, network.HostnameID, resource.VersionUndefined)

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

		r.ResetRestartBackoff()
	}
}
