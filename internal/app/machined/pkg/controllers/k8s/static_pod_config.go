// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// StaticPodConfigController manages k8s.StaticPod based on machine configuration.
type StaticPodConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *StaticPodConfigController) Name() string {
	return "k8s.StaticPodConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StaticPodConfigController) Inputs() []controller.Input {
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
func (ctrl *StaticPodConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.StaticPodType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *StaticPodConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		r.StartTrackingOutputs()

		if cfg != nil && cfg.Config().Machine() != nil {
			cfgProvider := cfg.Config()

			for _, pod := range cfgProvider.Machine().Pods() {
				var (
					name, namespace string
					ok              bool
				)

				name, ok, err = unstructured.NestedString(pod, "metadata", "name")
				if err != nil {
					return fmt.Errorf("error getting name from static pod: %w", err)
				}

				if !ok {
					return errors.New("name is missing in static pod metadata")
				}

				namespace, ok, err = unstructured.NestedString(pod, "metadata", "namespace")
				if err != nil {
					return fmt.Errorf("error getting namespace from static pod: %w", err)
				}

				if !ok {
					namespace = corev1.NamespaceDefault
				}

				id := fmt.Sprintf("%s-%s", namespace, name)

				if err = safe.WriterModify(ctx, r, k8s.NewStaticPod(k8s.NamespaceName, id), func(r *k8s.StaticPod) error {
					r.TypedSpec().Pod = pod

					return nil
				}); err != nil {
					return fmt.Errorf("error modifying resource: %w", err)
				}
			}
		}

		// clean up static pods which haven't been touched
		if err = safe.CleanupOutputs[*k8s.StaticPod](ctx, r); err != nil {
			return err
		}
	}
}
