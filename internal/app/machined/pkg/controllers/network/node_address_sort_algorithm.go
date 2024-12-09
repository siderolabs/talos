// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NodeAddressSortAlgorithmController manages NodeAddressSortAlgorithm based on configuration.
type NodeAddressSortAlgorithmController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeAddressSortAlgorithmController) Name() string {
	return "network.NodeAddressSortAlgorithmController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeAddressSortAlgorithmController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeAddressSortAlgorithmController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.NodeAddressSortAlgorithmType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *NodeAddressSortAlgorithmController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get %s: %w", config.MachineConfigType, err)
		}

		algorithm := nethelpers.AddressSortAlgorithmV1

		if cfg != nil && cfg.Config().Machine() != nil {
			algorithm = cfg.Provider().Machine().Features().NodeAddressSortAlgorithm()
		}

		if err = safe.WriterModify(ctx, r, network.NewNodeAddressSortAlgorithm(network.NamespaceName, network.NodeAddressSortAlgorithmID), func(res *network.NodeAddressSortAlgorithm) error {
			res.TypedSpec().Algorithm = algorithm

			return nil
		}); err != nil {
			return fmt.Errorf("failed to update %s: %w", network.NodeAddressSortAlgorithmType, err)
		}

		r.ResetRestartBackoff()
	}
}
