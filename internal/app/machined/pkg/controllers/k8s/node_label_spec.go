// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

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

		var nodeLabels map[string]string

		cfg, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			nodeLabels = cfg.Config().Machine().NodeLabels()
		}

		for key, value := range nodeLabels {
			if err = r.Modify(ctx, k8s.NewNodeLabelSpec(key), func(r resource.Resource) error {
				r.(*k8s.NodeLabelSpec).TypedSpec().Key = key
				r.(*k8s.NodeLabelSpec).TypedSpec().Value = value

				return nil
			}); err != nil {
				return fmt.Errorf("error updating node label spec: %w", err)
			}
		}

		labelSpecs, err := safe.ReaderList[*k8s.NodeLabelSpec](ctx, r, k8s.NewNodeLabelSpec("").Metadata())
		if err != nil {
			return fmt.Errorf("error getting node label specs: %w", err)
		}

		for iter := safe.IteratorFromList(labelSpecs); iter.Next(); {
			labelSpec := iter.Value()

			_, touched := nodeLabels[labelSpec.TypedSpec().Key]
			if touched {
				continue
			}

			if err = r.Destroy(ctx, labelSpec.Metadata()); err != nil {
				return fmt.Errorf("error destroying node label spec: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}
