// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	k8sadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/talos-systems/talos/pkg/kubernetes/kubelet"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

// KubeletStaticPodController renders static pod definitions and manages k8s.StaticPodStatus.
type KubeletStaticPodController struct{}

// Name implements controller.Controller interface.
func (ctrl *KubeletStaticPodController) Name() string {
	return "k8s.KubeletStaticPodController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubeletStaticPodController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        pointer.To(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.To("kubelet"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        pointer.To(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        pointer.To(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubeletStaticPodController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.StaticPodStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *KubeletStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var kubeletClient *kubelet.Client

	refreshTicker := time.NewTicker(15 * time.Second) // refresh kubelet pods status every 15 seconds
	defer refreshTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-refreshTicker.C:
			if kubeletClient != nil {
				if err := ctrl.refreshPodStatus(ctx, r, kubeletClient); err != nil {
					return fmt.Errorf("error refreshing pod status: %w", err)
				}
			}

			continue
		case <-r.EventCh():
		}

		kubeletResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "kubelet", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				kubeletClient = nil

				if err = ctrl.teardownStatuses(ctx, r); err != nil {
					return fmt.Errorf("error tearing down: %w", err)
				}

				continue
			}

			return err
		}

		if !kubeletResource.(*v1alpha1.Service).TypedSpec().Running {
			kubeletClient = nil

			if err = ctrl.teardownStatuses(ctx, r); err != nil {
				return fmt.Errorf("error tearing down: %w", err)
			}

			continue
		}

		// on worker nodes, there's no way to connect to the kubelet to fetch the pod status (only API server can do that)
		// on control plane nodes, use API servers' client kubelet certificate to fetch statuses
		rootSecretResource, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				kubeletClient = nil

				continue
			}

			return err
		}

		rootSecrets := rootSecretResource.(*secrets.KubernetesRoot).TypedSpec()

		secretsResource, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				kubeletClient = nil

				continue
			}

			return err
		}

		secrets := secretsResource.(*secrets.Kubernetes).TypedSpec()

		nodenameResource, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
		if err != nil {
			// nodename should exist if the kubelet is running
			return err
		}

		nodename := nodenameResource.(*k8s.Nodename).TypedSpec().Nodename

		kubeletClient, err = kubelet.NewClient(nodename, secrets.APIServerKubeletClient.Crt, secrets.APIServerKubeletClient.Key, rootSecrets.CA.Crt)
		if err != nil {
			return fmt.Errorf("error building kubelet client: %w", err)
		}
	}
}

func (ctrl *KubeletStaticPodController) teardownStatuses(ctx context.Context, r controller.Runtime) error {
	statuses, err := r.List(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodStatusType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing pod statuses: %w", err)
	}

	for _, status := range statuses.Items {
		// TODO: proper teardown sequence?
		if err = r.Destroy(ctx, status.Metadata()); err != nil {
			return fmt.Errorf("error destroying stale pod status: %w", err)
		}
	}

	return nil
}

func (ctrl *KubeletStaticPodController) refreshPodStatus(ctx context.Context, r controller.Runtime, kubeletClient *kubelet.Client) error {
	podList, err := kubeletClient.Pods(ctx)
	if err != nil {
		return fmt.Errorf("error fetching pod status: %w", err)
	}

	podsSeen := map[string]struct{}{}

	for _, pod := range podList.Items {
		pod := pod

		switch pod.Metadata.Annotations.ConfigSource {
		case "file":
			// static pod from a file source
		case "http":
			// static pod from an HTTP source
		default:
			// anything else is not a static pod, skip it
			continue
		}

		statusID := fmt.Sprintf("%s/%s", pod.Metadata.Namespace, pod.Metadata.Name)

		podsSeen[statusID] = struct{}{}

		if err = r.Modify(ctx, k8s.NewStaticPodStatus(k8s.NamespaceName, statusID), func(r resource.Resource) error {
			return k8sadapter.StaticPodStatus(r.(*k8s.StaticPodStatus)).SetStatus(&pod.Status)
		}); err != nil {
			return fmt.Errorf("error updating pod status: %w", err)
		}
	}

	statuses, err := r.List(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodStatusType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing pod statuses: %w", err)
	}

	for _, status := range statuses.Items {
		if _, exists := podsSeen[status.Metadata().ID()]; !exists {
			// TODO: proper teardown sequence?
			if err = r.Destroy(ctx, status.Metadata()); err != nil {
				return fmt.Errorf("error destroying stale pod status: %w", err)
			}
		}
	}

	return nil
}
