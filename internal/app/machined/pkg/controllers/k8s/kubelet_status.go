// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubeletStatusController publishes the non-sensitive part of the kubelet configuration.
type KubeletStatusController = transform.Controller[*k8s.KubeletSpec, *k8s.KubeletStatus]

// NewKubeletStatusController instantiates the controller.
func NewKubeletStatusController() *KubeletStatusController {
	return transform.NewController(
		transform.Settings[*k8s.KubeletSpec, *k8s.KubeletStatus]{
			Name: "k8s.KubeletStatusController",
			MapMetadataFunc: func(spec *k8s.KubeletSpec) *k8s.KubeletStatus {
				return k8s.NewKubeletStatus(k8s.NamespaceName, spec.Metadata().ID())
			},
			TransformFunc: func(_ context.Context, _ controller.Reader, _ *zap.Logger, spec *k8s.KubeletSpec, status *k8s.KubeletStatus) error {
				status.TypedSpec().Image = spec.TypedSpec().Image

				return nil
			},
		},
	)
}
