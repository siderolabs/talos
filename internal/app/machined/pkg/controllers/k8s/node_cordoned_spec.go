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
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// NodeCordonedSpecController manages node cordoned status based on configuration.
type NodeCordonedSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeCordonedSpecController) Name() string {
	return "k8s.NodeCordonedSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeCordonedSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MachineStatusType,
			ID:        optional.Some(runtime.MachineStatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeCordonedSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodeCordonedSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeCordonedSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		status, err := safe.ReaderGetByID[*runtime.MachineStatus](ctx, r, runtime.MachineStatusID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		var shouldCordon bool

		switch status.TypedSpec().Stage { //nolint:exhaustive
		case runtime.MachineStageShuttingDown, runtime.MachineStageUpgrading, runtime.MachineStageResetting:
			shouldCordon = true
		case runtime.MachineStageBooting, runtime.MachineStageRunning:
			shouldCordon = false
		default:
			// don't change cordoned status
			continue
		}

		if shouldCordon {
			if err = safe.WriterModify(ctx, r, k8s.NewNodeCordonedSpec(k8s.NodeCordonedID),
				func(k *k8s.NodeCordonedSpec) error {
					return nil
				}); err != nil {
				return fmt.Errorf("error updating node cordoned spec: %w", err)
			}
		} else {
			nodeCordoned, err := safe.ReaderListAll[*k8s.NodeCordonedSpec](ctx, r)
			if err != nil {
				return fmt.Errorf("error getting node cordoned specs: %w", err)
			}

			for res := range nodeCordoned.All() {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error destroying node cordoned spec: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
