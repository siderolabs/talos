// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// NodeLabelSpecController manages k8s.NodeLabelsConfig based on configuration.
type NodeLabelSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeLabelSpecController) Name() string {
	return "k8s.NodeLabelSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeLabelSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeLabelSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodeLabelSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeLabelSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting config: %w", err)
		}

		r.StartTrackingOutputs()

		var nodeLabels map[string]string

		if cfg != nil && cfg.Config().Machine() != nil {
			nodeLabels = cfg.Config().Machine().NodeLabels()

			if cfg.Config().Machine().Type().IsControlPlane() {
				if nodeLabels == nil {
					nodeLabels = map[string]string{}
				}

				nodeLabels[constants.LabelNodeRoleControlPlane] = ""
			}
		}

		for key, value := range nodeLabels {
			if err = safe.WriterModify(ctx, r, k8s.NewNodeLabelSpec(key), func(k *k8s.NodeLabelSpec) error {
				k.TypedSpec().Key = key
				k.TypedSpec().Value = value

				return nil
			}); err != nil {
				return fmt.Errorf("error updating node label spec: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*k8s.NodeLabelSpec](ctx, r); err != nil {
			return err
		}
	}
}
