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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/pkg/kubernetes/kubelet"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
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
			ID:        optional.Some(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("kubelet"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesDynamicCertsType,
			ID:        optional.Some(secrets.KubernetesDynamicCertsID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        optional.Some(secrets.KubernetesRootID),
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
//nolint:gocyclo
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

		kubeletService, err := safe.ReaderGet[*v1alpha1.Service](ctx, r, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "kubelet", resource.VersionUndefined))
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

		if !kubeletService.TypedSpec().Running {
			kubeletClient = nil

			if err = ctrl.teardownStatuses(ctx, r); err != nil {
				return fmt.Errorf("error tearing down: %w", err)
			}

			continue
		}

		// on worker nodes, there's no way to connect to the kubelet to fetch the pod status (only API server can do that)
		// on control plane nodes, use API servers' client kubelet certificate to fetch statuses
		rootSecrets, err := safe.ReaderGet[*secrets.KubernetesRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				kubeletClient = nil

				continue
			}

			return err
		}

		certsResource, err := safe.ReaderGet[*secrets.KubernetesDynamicCerts](
			ctx, r,
			resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesDynamicCertsType, secrets.KubernetesDynamicCertsID, resource.VersionUndefined),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				kubeletClient = nil

				continue
			}

			return err
		}

		certs := certsResource.TypedSpec()

		nodename, err := safe.ReaderGet[*k8s.Nodename](ctx, r, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
		if err != nil {
			// nodename should exist if the kubelet is running
			return err
		}

		kubeletClient, err = kubelet.NewClient(
			nodename.TypedSpec().Nodename,
			certs.APIServerKubeletClient.Crt,
			certs.APIServerKubeletClient.Key,
			rootSecrets.TypedSpec().CA.Crt,
		)
		if err != nil {
			return fmt.Errorf("error building kubelet client: %w", err)
		}

		r.ResetRestartBackoff()
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

		if err = safe.WriterModify(ctx, r, k8s.NewStaticPodStatus(k8s.NamespaceName, statusID), func(r *k8s.StaticPodStatus) error {
			return k8sadapter.StaticPodStatus(r).SetStatus(&pod.Status)
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
