// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
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
			ID:        pointer.To(cluster.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        pointer.To(cluster.LocalIdentity),
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
func (ctrl *KubernetesPushController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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
			discoveryConfig, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, cluster.ConfigType, cluster.ConfigID, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting discovery config: %w", err)
				}

				continue
			}

			if !discoveryConfig.(*cluster.Config).TypedSpec().RegistryKubernetesEnabled {
				continue
			}

			if err = conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
				return err
			}

			identity, err := r.Get(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.IdentityType, cluster.LocalIdentity, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting local identity: %w", err)
				}

				continue
			}

			localAffiliateID := identity.(*cluster.Identity).TypedSpec().NodeID

			if ctrl.localAffiliateID != localAffiliateID {
				ctrl.localAffiliateID = localAffiliateID

				if err = r.UpdateInputs(append(ctrl.Inputs(),
					controller.Input{
						Namespace: cluster.NamespaceName,
						Type:      cluster.AffiliateType,
						ID:        pointer.To(ctrl.localAffiliateID),
						Kind:      controller.InputWeak,
					},
				)); err != nil {
					return err
				}
			}

			affiliate, err := r.Get(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.AffiliateType, ctrl.localAffiliateID, resource.VersionUndefined))
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

			if err = registry.NewKubernetes(ctrl.kubernetesClient).Push(ctx, affiliate.(*cluster.Affiliate)); err != nil {
				// reset client connection
				ctrl.kubernetesClient.Close() //nolint:errcheck
				ctrl.kubernetesClient = nil

				return fmt.Errorf("error pushing to Kubernetes registry: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}
