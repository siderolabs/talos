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
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubernetesPushController pushes Affiliate resource to the Kubernetes registry.
type KubernetesPushController struct {
	localAffiliateID    resource.ID
	kubernetesClient    *kubernetes.Client
	clientKubeconfigVer string
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
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubeletKubeconfigType,
			ID:        optional.Some(k8s.KubeletKubeconfigID),
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

			client, err := ctrl.getKubernetesClient(ctx, r, logger)
			if err != nil {
				return err
			}

			if client == nil {
				continue
			}

			if err = registry.NewKubernetes(client).Push(ctx, affiliate); err != nil {
				// reset client connection
				ctrl.kubernetesClient.Close() //nolint:errcheck
				ctrl.kubernetesClient = nil

				return fmt.Errorf("error pushing to Kubernetes registry: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

// getKubernetesClient returns the cached Kubernetes client, rebuilding it whenever the
// kubelet credentials on disk have changed since the client was built.
//
// The credentials are read once when the client is created, so a client which outlives a
// kubelet client certificate rotation would keep presenting an expired certificate.
//
// It returns a nil client if the kubelet credentials are not available yet.
func (ctrl *KubernetesPushController) getKubernetesClient(ctx context.Context, r controller.Runtime, logger *zap.Logger) (*kubernetes.Client, error) {
	kubeconfigRes, err := safe.ReaderGetByID[*k8s.KubeletKubeconfig](ctx, r, k8s.KubeletKubeconfigID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			return nil, fmt.Errorf("error getting kubelet kubeconfig: %w", err)
		}

		return nil, nil
	}

	currentKubeconfigVer := kubeconfigRes.TypedSpec().Hash

	if ctrl.kubernetesClient != nil && ctrl.clientKubeconfigVer != currentKubeconfigVer {
		logger.Info(
			"kubelet credentials changed, rebuilding Kubernetes client",
			zap.String("old_hash", ctrl.clientKubeconfigVer),
			zap.String("new_hash", currentKubeconfigVer),
		)

		ctrl.kubernetesClient.Close() //nolint:errcheck
		ctrl.kubernetesClient = nil
	}

	if ctrl.kubernetesClient == nil {
		ctrl.kubernetesClient, err = kubernetes.NewClientFromKubeletKubeconfig()
		if err != nil {
			return nil, fmt.Errorf("error building kubernetes client: %w", err)
		}

		ctrl.clientKubeconfigVer = currentKubeconfigVer
	}

	return ctrl.kubernetesClient, nil
}
