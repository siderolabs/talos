// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/k8s"
	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/secrets"
	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/v1alpha1"
	"github.com/talos-systems/talos/pkg/kubernetes/kubelet"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// KubeletStaticPodController renders static pod definitions and manages k8s.StaticPodStatus.
type KubeletStaticPodController struct{}

// Name implements controller.Controller interface.
func (ctrl *KubeletStaticPodController) Name() string {
	return "k8s.KubeletStaticPodController"
}

// ManagedResources implements controller.Controller interface.
func (ctrl *KubeletStaticPodController) ManagedResources() (resource.Namespace, resource.Type) {
	return k8s.ControlPlaneNamespaceName, k8s.StaticPodStatusType
}

// Run implements controller.Controller interface.
//
//nolint: gocyclo
func (ctrl *KubeletStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if err := r.UpdateDependencies([]controller.Dependency{
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.StaticPodType,
			Kind:      controller.DependencyStrong,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("kubelet"),
			Kind:      controller.DependencyWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        pointer.ToString(secrets.KubernetesID),
			Kind:      controller.DependencyWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.BootstrapStatusType,
			ID:        pointer.ToString(v1alpha1.BootstrapStatusID),
			Kind:      controller.DependencyWeak,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

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

		if !kubeletResource.(*v1alpha1.Service).Running() {
			kubeletClient = nil

			if err = ctrl.teardownStatuses(ctx, r); err != nil {
				return fmt.Errorf("error tearing down: %w", err)
			}

			continue
		}

		secretsResources, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		secrets := secretsResources.(*secrets.Kubernetes).Secrets()

		bootstrapStatus, err := r.Get(ctx, v1alpha1.NewBootstrapStatus().Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if bootstrapStatus.(*v1alpha1.BootstrapStatus).Status().SelfHostedControlPlane {
			logger.Print("skipped as running self-hosted control plane")

			continue
		}

		kubeletClientCert, err := tls.X509KeyPair(secrets.APIServerKubeletClient.Crt, secrets.APIServerKubeletClient.Key)
		if err != nil {
			return fmt.Errorf("error loading apiserver kubelet client cert: %w", err)
		}

		kubeletClient, err = kubelet.NewClient(kubeletClientCert)
		if err != nil {
			return fmt.Errorf("error building kubelet client: %w", err)
		}

		staticPods, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.StaticPodType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing static pods: %w", err)
		}

		for _, staticPod := range staticPods.Items {
			switch staticPod.Metadata().Phase() {
			case resource.PhaseRunning:
				if err = ctrl.runPod(ctx, r, logger, staticPod.(*k8s.StaticPod)); err != nil {
					return fmt.Errorf("error running pod: %w", err)
				}
			case resource.PhaseTearingDown:
				if err = ctrl.teardownPod(logger, staticPod.(*k8s.StaticPod)); err != nil {
					return fmt.Errorf("error tearing down pod: %w", err)
				}
			}
		}
	}
}

func (ctrl *KubeletStaticPodController) runPod(ctx context.Context, r controller.Runtime, logger *log.Logger, staticPod *k8s.StaticPod) error {
	staticPodStatus := k8s.NewStaticPodStatus(staticPod.Metadata().Namespace(), staticPod.Metadata().ID())

	if err := r.AddFinalizer(ctx, staticPod.Metadata(), staticPodStatus.String()); err != nil {
		return err
	}

	renderedPod, err := yaml.Marshal(staticPod.Spec())
	if err != nil {
		return nil
	}

	podPath := filepath.Join(constants.ManifestsDirectory, fmt.Sprintf("%s.yaml", staticPod.Metadata().ID()))

	existingPod, err := ioutil.ReadFile(podPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	if bytes.Equal(renderedPod, existingPod) {
		return nil
	}

	logger.Printf("writing static pod %q", podPath)

	return ioutil.WriteFile(podPath, renderedPod, 0o600)
}

func (ctrl *KubeletStaticPodController) teardownPod(logger *log.Logger, staticPod *k8s.StaticPod) error {
	podPath := filepath.Join(constants.ManifestsDirectory, fmt.Sprintf("%s.yaml", staticPod.Metadata().ID()))

	_, err := os.Stat(podPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("error checking static pod status: %w", err)
	}

	logger.Printf("removing static pod %q", podPath)

	if err = os.Remove(podPath); err != nil {
		return fmt.Errorf("error removing static pod %q: %w", podPath, err)
	}

	return nil
}

func (ctrl *KubeletStaticPodController) teardownStatuses(ctx context.Context, r controller.Runtime) error {
	statuses, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.StaticPodStatusType, "", resource.VersionUndefined))
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
		if pod.OwnerReferences != nil {
			continue
		}

		pod := pod

		statusID := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

		podsSeen[statusID] = struct{}{}

		if err = r.Update(ctx, k8s.NewStaticPodStatus(k8s.ControlPlaneNamespaceName, statusID), func(r resource.Resource) error {
			r.(*k8s.StaticPodStatus).SetStatus(&pod.Status)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating pod status: %w", err)
		}
	}

	statuses, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.StaticPodStatusType, "", resource.VersionUndefined))
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
