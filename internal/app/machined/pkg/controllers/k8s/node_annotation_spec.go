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

	"github.com/siderolabs/talos/pkg/machinery/labels"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// NodeAnnotationSpecController manages k8s.NodeAnnotationsConfig based on configuration.
type NodeAnnotationSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeAnnotationSpecController) Name() string {
	return "k8s.NodeAnnotationSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeAnnotationSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.ExtensionStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeAnnotationSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodeAnnotationSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeAnnotationSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

		nodeAnnotations := map[string]string{}

		if cfg != nil && cfg.Config().Machine() != nil {
			for k, v := range cfg.Config().Machine().NodeAnnotations() {
				nodeAnnotations[k] = v
			}
		}

		if err = extensionsToNodeKV(
			ctx, r, nodeAnnotations,
			func(annotationValue string) bool {
				return labels.ValidateLabelValue(annotationValue) != nil
			},
		); err != nil {
			return fmt.Errorf("error converting extensions to node annotations: %w", err)
		}

		for key, value := range nodeAnnotations {
			if err = safe.WriterModify(ctx, r, k8s.NewNodeAnnotationSpec(key), func(k *k8s.NodeAnnotationSpec) error {
				k.TypedSpec().Key = key
				k.TypedSpec().Value = value

				return nil
			}); err != nil {
				return fmt.Errorf("error updating node label spec: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*k8s.NodeAnnotationSpec](ctx, r); err != nil {
			return err
		}
	}
}
