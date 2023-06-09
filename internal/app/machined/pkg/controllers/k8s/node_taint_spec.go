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
	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// NodeTaintSpecController manages k8s.NodeLabelsConfig based on configuration.
type NodeTaintSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeTaintSpecController) Name() string {
	return "k8s.NodeTaintSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeTaintSpecController) Inputs() []controller.Input {
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
func (ctrl *NodeTaintSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodeTaintSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeTaintSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		touched := map[string]struct{}{}

		if cfg.Config().Machine().Type().IsControlPlane() && !cfg.Config().Cluster().ScheduleOnControlPlanes() {
			touched[constants.LabelNodeRoleControlPlane] = struct{}{}

			if err = safe.WriterModify(ctx, r, k8s.NewNodeTaintSpec(constants.LabelNodeRoleControlPlane), func(k *k8s.NodeTaintSpec) error {
				k.TypedSpec().Key = constants.LabelNodeRoleControlPlane
				k.TypedSpec().Value = ""
				k.TypedSpec().Effect = string(v1.TaintEffectNoSchedule)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating node taint spec: %w", err)
			}
		}

		taintSpecs, err := safe.ReaderListAll[*k8s.NodeTaintSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error getting node taint specs: %w", err)
		}

		for iter := safe.IteratorFromList(taintSpecs); iter.Next(); {
			if _, touched := touched[iter.Value().Metadata().ID()]; touched {
				continue
			}

			if err = r.Destroy(ctx, iter.Value().Metadata()); err != nil {
				return fmt.Errorf("error destroying node taint spec: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}
