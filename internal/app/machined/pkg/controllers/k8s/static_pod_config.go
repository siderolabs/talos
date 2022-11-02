// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
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
			ID:        pointer.To(config.V1Alpha1ID),
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
func (ctrl *StaticPodConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		touchedIDs := map[string]struct{}{}

		if cfg != nil {
			cfgProvider := cfg.(*config.MachineConfig).Config()

			for _, pod := range cfgProvider.Machine().Pods() {
				name, ok, err := unstructured.NestedString(pod, "metadata", "name")
				if err != nil {
					return fmt.Errorf("error getting name from static pod: %w", err)
				}

				if !ok {
					return fmt.Errorf("name is missing in static pod metadata")
				}

				namespace, ok, err := unstructured.NestedString(pod, "metadata", "namespace")
				if err != nil {
					return fmt.Errorf("error getting namespace from static pod: %w", err)
				}

				if !ok {
					namespace = corev1.NamespaceDefault
				}

				id := fmt.Sprintf("%s-%s", namespace, name)

				if err = r.Modify(ctx, k8s.NewStaticPod(k8s.NamespaceName, id), func(r resource.Resource) error {
					r.(*k8s.StaticPod).TypedSpec().Pod = pod

					return nil
				}); err != nil {
					return fmt.Errorf("error modifying resource: %w", err)
				}

				touchedIDs[id] = struct{}{}
			}
		}

		// clean up static pods which haven't been touched
		{
			list, err := r.List(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodType, "", resource.VersionUndefined))
			if err != nil {
				return err
			}

			for _, res := range list.Items {
				if _, ok := touchedIDs[res.Metadata().ID()]; ok {
					continue
				}

				if res.Metadata().Owner() != ctrl.Name() {
					continue
				}

				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return err
				}
			}
		}
	}
}
