// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v3"

	k8sadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/talos-systems/talos/pkg/kubernetes/kubelet"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.StaticPodType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        pointer.ToString(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("kubelet"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        pointer.ToString(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        pointer.ToString(secrets.KubernetesRootID),
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

		if !kubeletResource.(*v1alpha1.Service).Running() {
			kubeletClient = nil

			if err = ctrl.teardownStatuses(ctx, r); err != nil {
				return fmt.Errorf("error tearing down: %w", err)
			}

			continue
		}

		rootSecretResource, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.cleanupPods(logger, nil); err != nil {
					return fmt.Errorf("error cleaning up static pods: %w", err)
				}

				continue
			}

			return err
		}

		rootSecrets := rootSecretResource.(*secrets.KubernetesRoot).TypedSpec()

		secretsResource, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.cleanupPods(logger, nil); err != nil {
					return fmt.Errorf("error cleaning up static pods: %w", err)
				}

				continue
			}

			return err
		}

		secrets := secretsResource.(*secrets.Kubernetes).Certs()

		nodenameResource, err := r.Get(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
		if err != nil {
			// nodename should exist if the kubelet is running
			return err
		}

		nodename := nodenameResource.(*k8s.Nodename).TypedSpec().Nodename

		staticPods, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.StaticPodType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing static pods: %w", err)
		}

		for _, staticPod := range staticPods.Items {
			switch staticPod.Metadata().Phase() {
			case resource.PhaseRunning:
				if err = ctrl.writePod(logger, staticPod); err != nil {
					return fmt.Errorf("error running pod: %w", err)
				}
			case resource.PhaseTearingDown:
				if err = ctrl.teardownPod(logger, staticPod); err != nil {
					return fmt.Errorf("error tearing down pod: %w", err)
				}
			}
		}

		if err = ctrl.cleanupPods(logger, staticPods.Items); err != nil {
			return fmt.Errorf("error cleaning up static pods: %w", err)
		}

		// render static pods first, and attempt to build kubelet client last,
		// as if kubelet issues certs from the API server, API server should be launched first.
		kubeletClient, err = kubelet.NewClient(nodename, secrets.APIServerKubeletClient.Crt, secrets.APIServerKubeletClient.Key, rootSecrets.CA.Crt)
		if err != nil {
			return fmt.Errorf("error building kubelet client: %w", err)
		}
	}
}

func (ctrl *KubeletStaticPodController) podPath(staticPod resource.Resource) string {
	return filepath.Join(constants.ManifestsDirectory, ctrl.podFilename(staticPod))
}

func (ctrl *KubeletStaticPodController) podFilename(staticPod resource.Resource) string {
	return fmt.Sprintf("%s%s.yaml", constants.TalosManifestPrefix, staticPod.Metadata().ID())
}

func (ctrl *KubeletStaticPodController) writePod(logger *zap.Logger, staticPod resource.Resource) error {
	renderedPod, err := yaml.Marshal(staticPod.Spec())
	if err != nil {
		return err
	}

	podPath := ctrl.podPath(staticPod)

	existingPod, err := ioutil.ReadFile(podPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}

	if bytes.Equal(renderedPod, existingPod) {
		return nil
	}

	logger.Sugar().Infof("writing static pod %q", podPath)

	return ioutil.WriteFile(podPath, renderedPod, 0o600)
}

func (ctrl *KubeletStaticPodController) teardownPod(logger *zap.Logger, staticPod resource.Resource) error {
	podPath := ctrl.podPath(staticPod)

	_, err := os.Stat(podPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("error checking static pod status: %w", err)
	}

	logger.Sugar().Infof("removing static pod %q", podPath)

	if err = os.Remove(podPath); err != nil {
		return fmt.Errorf("error removing static pod %q: %w", podPath, err)
	}

	return nil
}

func (ctrl *KubeletStaticPodController) cleanupPods(logger *zap.Logger, staticPods []resource.Resource) error {
	manifestDir, err := os.Open(constants.ManifestsDirectory)
	if err != nil {
		return fmt.Errorf("error opening manifests directory: %w", err)
	}

	defer manifestDir.Close() //nolint:errcheck

	manifests, err := manifestDir.Readdirnames(0)
	if err != nil {
		return fmt.Errorf("error listing manifests: %w", err)
	}

	expectedManifests := map[string]struct{}{}

	for _, staticPod := range staticPods {
		expectedManifests[ctrl.podFilename(staticPod)] = struct{}{}
	}

	for _, manifest := range manifests {
		// skip manifests
		if !strings.HasPrefix(manifest, constants.TalosManifestPrefix) {
			continue
		}

		if _, expected := expectedManifests[manifest]; expected {
			continue
		}

		podPath := filepath.Join(constants.ManifestsDirectory, manifest)

		logger.Sugar().Infof("cleaning up static pod %q", podPath)

		if err = os.Remove(podPath); err != nil {
			return fmt.Errorf("error cleaning up static pod: %w", err)
		}
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

		if err = r.Modify(ctx, k8s.NewStaticPodStatus(k8s.ControlPlaneNamespaceName, statusID), func(r resource.Resource) error {
			return k8sadapter.StaticPodStatus(r.(*k8s.StaticPodStatus)).SetStatus(&pod.Status)
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
