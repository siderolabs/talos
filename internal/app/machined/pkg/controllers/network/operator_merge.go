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

	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// OperatorMergeController merges network.OperatorSpec in network.ConfigNamespace and produces final network.OperatorSpec in network.Namespace.
type OperatorMergeController struct{}

// Name implements controller.Controller interface.
func (ctrl *OperatorMergeController) Name() string {
	return "network.OperatorMergeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *OperatorMergeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.ConfigNamespaceName,
			Type:      network.OperatorSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.OperatorSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *OperatorMergeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.OperatorSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *OperatorMergeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network operators: %w", err)
		}

		// operator is allowed as long as it's not duplicate, for duplicate higher layer takes precedence
		operators := map[string]*network.OperatorSpec{}

		for _, res := range list.Items {
			operator := res.(*network.OperatorSpec) //nolint:errcheck,forcetypeassert
			id := network.OperatorID(operator.TypedSpec().Operator, operator.TypedSpec().LinkName)

			existing, ok := operators[id]
			if ok && existing.TypedSpec().ConfigLayer > operator.TypedSpec().ConfigLayer {
				// skip this operator, as existing one is higher layer
				continue
			}

			operators[id] = operator
		}

		conflictsDetected := 0

		for id, operator := range operators {
			operator := operator

			if err = r.Modify(ctx, network.NewOperatorSpec(network.NamespaceName, id), func(res resource.Resource) error {
				op := res.(*network.OperatorSpec) //nolint:errcheck,forcetypeassert

				*op.TypedSpec() = *operator.TypedSpec()

				return nil
			}); err != nil {
				if state.IsPhaseConflictError(err) {
					// phase conflict, resource is being torn down, skip updating it and trigger reconcile
					// later by failing the loop after all processing is done
					conflictsDetected++

					delete(operators, id)
				} else {
					return fmt.Errorf("error updating resource: %w", err)
				}
			}
		}

		// list operators for cleanup
		list, err = r.List(ctx, resource.NewMetadata(network.NamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if _, ok := operators[res.Metadata().ID()]; !ok {
				var okToDestroy bool

				okToDestroy, err = r.Teardown(ctx, res.Metadata())
				if err != nil {
					return fmt.Errorf("error cleaning up operators: %w", err)
				}

				if okToDestroy {
					if err = r.Destroy(ctx, res.Metadata()); err != nil {
						return fmt.Errorf("error cleaning up operators: %w", err)
					}
				}
			}
		}

		if conflictsDetected > 0 {
			return fmt.Errorf("%d conflict(s) detected", conflictsDetected)
		}
	}
}
