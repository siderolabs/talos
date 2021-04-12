// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-retry/retry"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/client"
	v1alpha1config "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	machinetype "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/config"
)

// UpgradeProvider are the cluster interfaces required by upgrade process.
type UpgradeProvider interface {
	cluster.ClientProvider
	cluster.K8sProvider
}

// UpgradeTalosManaged the Kubernetes control plane.
//
//nolint:gocyclo
func UpgradeTalosManaged(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) error {
	switch path := options.Path(); path {
	case "1.19->1.19":
		// nothing

	case "1.19->1.20":
		options.extraUpdaters = append(options.extraUpdaters, addControlPlaneToleration())

	// nothing for all those
	case "1.20->1.20":
	case "1.20->1.21":
	case "1.21->1.21":

	default:
		return fmt.Errorf("unsupported upgrade path %q (from %q to %q)", path, options.FromVersion, options.ToVersion)
	}

	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	options.masterNodes, err = k8sClient.NodeIPs(ctx, machinetype.TypeControlPlane)
	if err != nil {
		return fmt.Errorf("error fetching master nodes: %w", err)
	}

	if len(options.masterNodes) == 0 {
		return fmt.Errorf("no master nodes discovered")
	}

	fmt.Printf("discovered master nodes %q\n", options.masterNodes)

	for _, service := range []string{kubeAPIServer, kubeControllerManager, kubeScheduler} {
		if err = upgradeConfigPatch(ctx, cluster, options, service); err != nil {
			return fmt.Errorf("failed updating service %q: %w", service, err)
		}
	}

	if err = hyperkubeUpgradeDs(ctx, k8sClient.Clientset, kubeProxy, options); err != nil {
		return fmt.Errorf("error updating kube-proxy: %w", err)
	}

	return nil
}

func upgradeConfigPatch(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions, service string) error {
	fmt.Printf("updating %q to version %q\n", service, options.ToVersion)

	for _, node := range options.masterNodes {
		if err := upgradeNodeConfigPatch(ctx, cluster, options, service, node); err != nil {
			return fmt.Errorf("error updating node %q: %w", node, err)
		}
	}

	return nil
}

//nolint:gocyclo
func upgradeNodeConfigPatch(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions, service, node string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNodes(ctx, node)

	fmt.Printf(" > updating node %q\n", node)

	watchClient, err := c.Resources.Watch(ctx, config.NamespaceName, config.K8sControlPlaneType, service)
	if err != nil {
		return fmt.Errorf("error watching service configuration: %w", err)
	}

	// first response is resource definition
	_, err = watchClient.Recv()
	if err != nil {
		return fmt.Errorf("error watching config: %w", err)
	}

	// second is the initial state
	watchInitial, err := watchClient.Recv()
	if err != nil {
		return fmt.Errorf("error watching config: %w", err)
	}

	if watchInitial.EventType != state.Created {
		return fmt.Errorf("unexpected event type: %d", watchInitial.EventType)
	}

	skipConfigWait := false

	err = patchNodeConfig(ctx, cluster, node, upgradeConfigPatcher(options, service, watchInitial.Resource))
	if err != nil {
		if errors.Is(err, errUpdateSkipped) {
			skipConfigWait = true
		} else {
			return fmt.Errorf("error patching node config: %w", err)
		}
	}

	var expectedConfigVersion string

	if !skipConfigWait {
		watchUpdated, err := watchClient.Recv()
		if err != nil {
			return fmt.Errorf("error watching config: %w", err)
		}

		if watchUpdated.EventType != state.Updated {
			return fmt.Errorf("unexpected event type: %d", watchInitial.EventType)
		}

		expectedConfigVersion = watchUpdated.Resource.Metadata().Version().String()
	} else {
		expectedConfigVersion = watchInitial.Resource.Metadata().Version().String()
	}

	return retry.Constant(3*time.Minute, retry.WithUnits(10*time.Second), retry.WithErrorLogging(true)).Retry(func() error {
		return checkPodStatus(ctx, cluster, service, node, expectedConfigVersion)
	})
}

var errUpdateSkipped = fmt.Errorf("update skipped")

//nolint:gocyclo
func upgradeConfigPatcher(options UpgradeOptions, service string, configResource resource.Resource) func(config *v1alpha1config.Config) error {
	return func(config *v1alpha1config.Config) error {
		if config.ClusterConfig == nil {
			config.ClusterConfig = &v1alpha1config.ClusterConfig{}
		}

		configData := configResource.(*resource.Any).Value().(map[string]interface{}) //nolint:errcheck,forcetypeassert
		configImage := configData["image"].(string)                                   //nolint:errcheck,forcetypeassert

		switch service {
		case kubeAPIServer:
			if config.ClusterConfig.APIServerConfig == nil {
				config.ClusterConfig.APIServerConfig = &v1alpha1config.APIServerConfig{}
			}

			image := fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, options.ToVersion)

			if config.ClusterConfig.APIServerConfig.ContainerImage == image || configImage == image {
				return errUpdateSkipped
			}

			config.ClusterConfig.APIServerConfig.ContainerImage = image
		case kubeControllerManager:
			if config.ClusterConfig.ControllerManagerConfig == nil {
				config.ClusterConfig.ControllerManagerConfig = &v1alpha1config.ControllerManagerConfig{}
			}

			image := fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, options.ToVersion)

			if config.ClusterConfig.ControllerManagerConfig.ContainerImage == image || configImage == image {
				return errUpdateSkipped
			}

			config.ClusterConfig.ControllerManagerConfig.ContainerImage = image
		case kubeScheduler:
			if config.ClusterConfig.SchedulerConfig == nil {
				config.ClusterConfig.SchedulerConfig = &v1alpha1config.SchedulerConfig{}
			}

			image := fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, options.ToVersion)

			if config.ClusterConfig.SchedulerConfig.ContainerImage == image || configImage == image {
				return errUpdateSkipped
			}

			config.ClusterConfig.SchedulerConfig.ContainerImage = image
		default:
			return fmt.Errorf("unsupported service %q", service)
		}

		return nil
	}
}

//nolint:gocyclo
func checkPodStatus(ctx context.Context, cluster UpgradeProvider, service, node, configVersion string) error {
	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return retry.UnexpectedError(fmt.Errorf("error building kubernetes client: %w", err))
	}

	pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{
		LabelSelector: fmt.Sprintf("k8s-app = %s", service),
	})
	if err != nil {
		if kubernetes.IsRetryableError(err) {
			return retry.ExpectedError(err)
		}

		return retry.UnexpectedError(err)
	}

	podFound := false

	for _, pod := range pods.Items {
		if pod.Status.HostIP != node {
			continue
		}

		podFound = true

		if pod.Annotations[constants.AnnotationStaticPodConfigVersion] != configVersion {
			return retry.ExpectedError(fmt.Errorf("config version mismatch: got %q, expected %q", pod.Annotations[constants.AnnotationStaticPodConfigVersion], configVersion))
		}

		ready := false

		for _, condition := range pod.Status.Conditions {
			if condition.Type != "Ready" {
				continue
			}

			if condition.Status == "True" {
				ready = true

				break
			}
		}

		if !ready {
			return retry.ExpectedError(fmt.Errorf("pod is not ready"))
		}

		break
	}

	if !podFound {
		return retry.ExpectedError(fmt.Errorf("pod not found in the API server state"))
	}

	return nil
}
