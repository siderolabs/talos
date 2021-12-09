// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/internal/pkg/discovery/registry"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// KubernetesPullController pulls list of Affiliate resource from the Kubernetes registry.
type KubernetesPullController struct{}

// Name implements controller.Controller interface.
func (ctrl *KubernetesPullController) Name() string {
	return "cluster.KubernetesPullController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubernetesPullController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      cluster.ConfigType,
			ID:        pointer.ToString(cluster.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        pointer.ToString(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubernetesPullController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.AffiliateType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *KubernetesPullController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		kubernetesClient   *kubernetes.Client
		kubernetesRegistry *registry.Kubernetes
		watchCtxCancel     context.CancelFunc
		notifyCh           <-chan struct{}
	)

	defer func() {
		if watchCtxCancel != nil {
			watchCtxCancel()
		}

		if kubernetesClient != nil {
			kubernetesClient.Close() //nolint:errcheck
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-notifyCh:
		}

		discoveryConfig, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, cluster.ConfigType, cluster.ConfigID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting discovery config: %w", err)
			}

			continue
		}

		if !discoveryConfig.(*cluster.Config).TypedSpec().RegistryKubernetesEnabled {
			// if discovery is disabled cleanup existing resources
			if err = cleanupAffiliates(ctx, ctrl, r, nil); err != nil {
				return err
			}

			continue
		}

		if err = conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
			return err
		}

		nodename, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting nodename: %w", err)
			}

			continue
		}

		if kubernetesClient == nil {
			kubernetesClient, err = kubernetes.NewClientFromKubeletKubeconfig()
			if err != nil {
				return fmt.Errorf("error building kubernetes client: %w", err)
			}
		}

		if kubernetesRegistry == nil {
			kubernetesRegistry = registry.NewKubernetes(kubernetesClient)
		}

		if notifyCh == nil {
			var watchCtx context.Context
			watchCtx, watchCtxCancel = context.WithCancel(ctx) //nolint:govet

			notifyCh, err = kubernetesRegistry.Watch(watchCtx, logger)
			if err != nil {
				return fmt.Errorf("error setting up registry watcher: %w", err) //nolint:govet
			}
		}

		affiliateSpecs, err := kubernetesRegistry.List(nodename.(*k8s.Nodename).TypedSpec().Nodename)
		if err != nil {
			return fmt.Errorf("error listing affiliates: %w", err)
		}

		touchedIDs := make(map[resource.ID]struct{})

		for _, affilateSpec := range affiliateSpecs {
			id := fmt.Sprintf("k8s/%s", affilateSpec.NodeID)

			affilateSpec := affilateSpec

			if err = r.Modify(ctx, cluster.NewAffiliate(cluster.RawNamespaceName, id), func(res resource.Resource) error {
				*res.(*cluster.Affiliate).TypedSpec() = *affilateSpec

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[id] = struct{}{}
		}

		if err := cleanupAffiliates(ctx, ctrl, r, touchedIDs); err != nil {
			return err
		}
	}
}
