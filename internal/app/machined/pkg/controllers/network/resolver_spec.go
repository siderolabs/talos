// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

// ResolverSpecController applies network.ResolverSpec to the actual interfaces.
type ResolverSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *ResolverSpecController) Name() string {
	return "network.ResolverSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ResolverSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.ResolverSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ResolverSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.ResolverStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ResolverSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.ResolverSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// add finalizers for all live resources
		for _, res := range list.Items {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		// loop over specs and sync to statuses
		for _, res := range list.Items {
			spec := res.(*network.ResolverSpec) //nolint:forcetypeassert,errcheck

			switch spec.Metadata().Phase() {
			case resource.PhaseTearingDown:
				if err = r.Destroy(ctx, resource.NewMetadata(network.NamespaceName, network.ResolverStatusType, spec.Metadata().ID(), resource.VersionUndefined)); err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error destroying status: %w", err)
				}

				if err = r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer: %w", err)
				}
			case resource.PhaseRunning:
				logger.Info("setting resolvers", zap.Stringers("resolvers", spec.TypedSpec().DNSServers))

				if err = r.Modify(ctx, network.NewResolverStatus(network.NamespaceName, spec.Metadata().ID()), func(r resource.Resource) error {
					status := r.(*network.ResolverStatus) //nolint:forcetypeassert,errcheck

					status.TypedSpec().DNSServers = spec.TypedSpec().DNSServers

					return nil
				}); err != nil {
					return fmt.Errorf("error modifying status: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
