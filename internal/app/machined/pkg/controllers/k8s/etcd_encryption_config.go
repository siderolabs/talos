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

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8stemplates"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// EtcdEncryptionConfigController renders kube-apiserver etcd encryption config.
type EtcdEncryptionConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *EtcdEncryptionConfigController) Name() string {
	return "k8s.EtcdEncryptionConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EtcdEncryptionConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        optional.Some(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EtcdEncryptionConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.EtcdEncryptionConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EtcdEncryptionConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		k8sRoot, err := safe.ReaderGetByID[*secrets.KubernetesRoot](ctx, r, secrets.KubernetesRootID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting KubernetesRoot secrets: %w", err)
		}

		if k8sRoot != nil {
			cfg, err := k8stemplates.APIServerEncryptionConfig(k8sRoot.TypedSpec())
			if err != nil {
				return fmt.Errorf("error generating API server encryption config: %w", err)
			}

			marshaled, err := k8stemplates.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("error marshaling API server encryption config: %w", err)
			}

			if err = safe.WriterModify(
				ctx,
				r,
				k8s.NewEtcdEncryptionConfig(k8s.EtcdEncryptionConfigID),
				func(r *k8s.EtcdEncryptionConfig) error {
					r.TypedSpec().Configuration = string(marshaled)

					return nil
				},
			); err != nil {
				return fmt.Errorf("error modifying EtcdEncryptionConfig resource: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*k8s.EtcdEncryptionConfig](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up etcd encryption config: %w", err)
		}
	}
}
