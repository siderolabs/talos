// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/discovery/registry"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// KubernetesPushController pushes Affiliate resource to the Kubernetes registry.
type KubernetesPushController struct {
	localAffiliateID resource.ID
	kubernetesClient *kubernetes.Client
}

// Name implements controller.Controller interface.
func (ctrl *KubernetesPushController) Name() string {
	return "cluster.KubernetesPushController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubernetesPushController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      cluster.ConfigType,
			ID:        optional.Some(cluster.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        optional.Some(cluster.LocalIdentity),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubernetesPushController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KubernetesPushController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	defer func() {
		if ctrl.kubernetesClient != nil {
			ctrl.kubernetesClient.Close() //nolint:errcheck
		}

		ctrl.kubernetesClient = nil
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			discoveryConfig, err := safe.ReaderGetByID[*cluster.Config](ctx, r, cluster.ConfigID)
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting discovery config: %w", err)
				}

				continue
			}

			if !discoveryConfig.TypedSpec().RegistryKubernetesEnabled {
				continue
			}

			if err = conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
				return err
			}

			identity, err := safe.ReaderGetByID[*cluster.Identity](ctx, r, cluster.LocalIdentity)
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting local identity: %w", err)
				}

				continue
			}

			localAffiliateID := identity.TypedSpec().NodeID

			if ctrl.localAffiliateID != localAffiliateID {
				ctrl.localAffiliateID = localAffiliateID

				if err = r.UpdateInputs(append(ctrl.Inputs(),
					controller.Input{
						Namespace: cluster.NamespaceName,
						Type:      cluster.AffiliateType,
						ID:        optional.Some(ctrl.localAffiliateID),
						Kind:      controller.InputWeak,
					},
				)); err != nil {
					return err
				}
			}

			affiliate, err := safe.ReaderGetByID[*cluster.Affiliate](ctx, r, ctrl.localAffiliateID)
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting local affiliate: %w", err)
				}

				continue
			}

			if ctrl.kubernetesClient == nil {
				ctrl.kubernetesClient, err = kubernetes.NewClientFromKubeletKubeconfig()
				if err != nil {
					return fmt.Errorf("error building kubernetes client: %w", err)
				}
			}

			if err = registry.NewKubernetes(ctrl.kubernetesClient).Push(ctx, affiliate); err != nil {
				// reset client connection
				ctrl.kubernetesClient.Close() //nolint:errcheck
				ctrl.kubernetesClient = nil

				return fmt.Errorf("error pushing to Kubernetes registry: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}
