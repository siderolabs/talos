// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	machinetype "github.com/siderolabs/talos/pkg/machinery/config/machine"
	v1alpha1config "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// UpgradeProvider are the cluster interfaces required by upgrade process.
type UpgradeProvider interface {
	cluster.ClientProvider
	cluster.K8sProvider
}

// Upgrade the Kubernetes control plane components, manifests, kubelets.
//
//nolint:gocyclo
func Upgrade(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) error {
	if !options.Path.IsSupported() {
		return fmt.Errorf("unsupported upgrade path %s (from %q to %q)", options.Path, options.Path.FromVersion(), options.Path.ToVersion())
	}

	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	defer k8sClient.Close() //nolint:errcheck

	options.controlPlaneNodes, err = k8sClient.NodeIPs(ctx, machinetype.TypeControlPlane)
	if err != nil {
		return fmt.Errorf("error fetching controlplane nodes: %w", err)
	}

	if len(options.controlPlaneNodes) == 0 {
		return fmt.Errorf("no controlplane nodes discovered")
	}

	options.Log("discovered controlplane nodes %q", options.controlPlaneNodes)

	if options.UpgradeKubelet {
		options.workerNodes, err = k8sClient.NodeIPs(ctx, machinetype.TypeWorker)
		if err != nil {
			return fmt.Errorf("error fetching worker nodes: %w", err)
		}

		options.Log("discovered worker nodes %q", options.workerNodes)
	}

	talosClient, err := cluster.Client()
	if err != nil {
		return err
	}

	k8sConfig, err := cluster.K8sRestConfig(ctx)
	if err != nil {
		return err
	}

	upgradeChecks, err := upgrade.NewChecks(options.Path, talosClient.COSI, k8sConfig, options.controlPlaneNodes, options.workerNodes, options.Log)
	if err != nil {
		return err
	}

	if err = upgradeChecks.Run(ctx); err != nil {
		return err
	}

	if options.PrePullImages {
		if err = prePullImages(ctx, talosClient, options); err != nil {
			return fmt.Errorf("failed pre-pulling images: %w", err)
		}
	}

	for _, service := range []string{kubeAPIServer, kubeControllerManager, kubeScheduler} {
		if err = upgradeStaticPod(ctx, cluster, options, service); err != nil {
			return fmt.Errorf("failed updating service %q: %w", service, err)
		}
	}

	if err = upgradeKubeProxy(ctx, cluster, options); err != nil {
		return fmt.Errorf("failed updating kube-proxy: %w", err)
	}

	if err = upgradeKubelet(ctx, cluster, options); err != nil {
		return fmt.Errorf("failed upgrading kubelet: %w", err)
	}

	objects, err := getManifests(ctx, cluster)
	if err != nil {
		return err
	}

	return syncManifests(ctx, objects, cluster, options)
}

func prePullImages(ctx context.Context, talosClient *client.Client, options UpgradeOptions) error {
	for _, imageRef := range []string{
		fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, options.Path.ToVersion()),
		fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, options.Path.ToVersion()),
		fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, options.Path.ToVersion()),
	} {
		for _, node := range options.controlPlaneNodes {
			options.Log(" > %q: pre-pulling %s", node, imageRef)

			err := talosClient.ImagePull(client.WithNode(ctx, node), common.ContainerdNamespace_NS_CRI, imageRef)
			if err != nil {
				if status.Code(err) == codes.Unimplemented {
					options.Log(" < %q: not implemented, skipping", node)
				} else {
					return fmt.Errorf("error pre-pulling %s on %s: %w", imageRef, node, err)
				}
			}
		}
	}

	if !options.UpgradeKubelet {
		return nil
	}

	imageRef := fmt.Sprintf("%s:v%s", constants.KubeletImage, options.Path.ToVersion())

	for _, node := range append(append([]string(nil), options.controlPlaneNodes...), options.workerNodes...) {
		options.Log(" > %q: pre-pulling %s", node, imageRef)

		err := talosClient.ImagePull(client.WithNode(ctx, node), common.ContainerdNamespace_NS_SYSTEM, imageRef)
		if err != nil {
			if status.Code(err) == codes.Unimplemented {
				options.Log(" < %q: not implemented, skipping", node)
			} else {
				return fmt.Errorf("error pre-pulling %s on %s: %w", imageRef, node, err)
			}
		}
	}

	return nil
}

func upgradeStaticPod(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions, service string) error {
	options.Log("updating %q to version %q", service, options.Path.ToVersion())

	for _, node := range options.controlPlaneNodes {
		if err := upgradeStaticPodOnNode(ctx, cluster, options, service, node); err != nil {
			return fmt.Errorf("error updating node %q: %w", node, err)
		}
	}

	return nil
}

func upgradeKubeProxy(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) error {
	options.Log("updating kube-proxy to version %q", options.Path.ToVersion())

	for _, node := range options.controlPlaneNodes {
		options.Log(" > %q: starting update", node)

		if err := patchNodeConfig(ctx, cluster, node, options.EncoderOpt, patchKubeProxy(options)); err != nil {
			return fmt.Errorf("error updating node %q: %w", node, err)
		}
	}

	return nil
}

func patchKubeProxy(options UpgradeOptions) func(config *v1alpha1config.Config) error {
	return func(config *v1alpha1config.Config) error {
		if options.DryRun {
			options.Log(" > skipped in dry-run")

			return nil
		}

		if config.ClusterConfig == nil {
			config.ClusterConfig = &v1alpha1config.ClusterConfig{}
		}

		if config.ClusterConfig.ProxyConfig == nil {
			config.ClusterConfig.ProxyConfig = &v1alpha1config.ProxyConfig{}
		}

		config.ClusterConfig.ProxyConfig.ContainerImage = fmt.Sprintf("%s:v%s", constants.KubeProxyImage, options.Path.ToVersion())

		return nil
	}
}

func controlplaneConfigResourceType(service string) resource.Type {
	switch service {
	case kubeAPIServer:
		return k8s.APIServerConfigType
	case kubeControllerManager:
		return k8s.ControllerManagerConfigType
	case kubeScheduler:
		return k8s.SchedulerConfigType
	}

	panic(fmt.Sprintf("unknown service ID %q", service))
}

//nolint:gocyclo
func upgradeStaticPodOnNode(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions, service, node string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNode(ctx, node)

	options.Log(" > %q: starting update", node)

	watchCh := make(chan state.Event)

	if err = c.COSI.Watch(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, controlplaneConfigResourceType(service), service, resource.VersionUndefined), watchCh); err != nil {
		return fmt.Errorf("error watching service configuration: %w", err)
	}

	var (
		expectedConfigVersion string
		initialConfig         resource.Resource
	)

	select {
	case ev := <-watchCh:
		if ev.Type != state.Created {
			return fmt.Errorf("unexpected event type: %d", ev.Type)
		}

		expectedConfigVersion = ev.Resource.Metadata().Version().String()
		initialConfig = ev.Resource
	case <-ctx.Done():
		return ctx.Err()
	}

	skipConfigWait := false

	err = patchNodeConfig(ctx, cluster, node, options.EncoderOpt, upgradeStaticPodPatcher(options, service, initialConfig))
	if err != nil {
		if errors.Is(err, errUpdateSkipped) {
			skipConfigWait = true
		} else {
			return fmt.Errorf("error patching node config: %w", err)
		}
	}

	if options.DryRun {
		return nil
	}

	options.Log(" > %q: machine configuration patched", node)
	options.Log(" > %q: waiting for %s pod update", node, service)

	if !skipConfigWait {
		select {
		case ev := <-watchCh:
			if ev.Type != state.Updated {
				return fmt.Errorf("unexpected event type: %d", ev.Type)
			}

			expectedConfigVersion = ev.Resource.Metadata().Version().String()
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err = checkPodStatus(ctx, cluster, options, service, node, expectedConfigVersion); err != nil {
		return err
	}

	options.Log(" < %q: successfully updated", node)

	return nil
}

var errUpdateSkipped = fmt.Errorf("update skipped")

//nolint:gocyclo,cyclop
func upgradeStaticPodPatcher(options UpgradeOptions, service string, configResource resource.Resource) func(config *v1alpha1config.Config) error {
	return func(config *v1alpha1config.Config) error {
		if config.ClusterConfig == nil {
			config.ClusterConfig = &v1alpha1config.ClusterConfig{}
		}

		var configImage string

		switch r := configResource.(type) {
		case *k8s.APIServerConfig:
			configImage = r.TypedSpec().Image
		case *k8s.ControllerManagerConfig:
			configImage = r.TypedSpec().Image
		case *k8s.SchedulerConfig:
			configImage = r.TypedSpec().Image
		default:
			return fmt.Errorf("unsupported service config %T", configResource)
		}

		logUpdate := func(oldImage string) {
			parts := strings.Split(oldImage, ":")
			version := options.Path.FromVersion()

			if oldImage == "" {
				version = options.Path.FromVersion()
			}

			if len(parts) > 1 {
				version = parts[1]
			}

			options.Log(" > update %s: %s -> %s", service, version, options.Path.ToVersion())

			if options.DryRun {
				options.Log(" > skipped in dry-run")
			}
		}

		switch service {
		case kubeAPIServer:
			if config.ClusterConfig.APIServerConfig == nil {
				config.ClusterConfig.APIServerConfig = &v1alpha1config.APIServerConfig{}
			}

			imageName := constants.KubernetesAPIServerImage

			privateRepo := strings.Split(config.ClusterConfig.APIServerConfig.ContainerImage, ":")[0]
			if privateRepo != constants.KubernetesAPIServerImage && !options.PrePullImages {
				imageName = privateRepo
			}

			image := fmt.Sprintf("%s:v%s", imageName, options.Path.ToVersion())
			options.Log(" > using apiServerImage %s", image)

			if config.ClusterConfig.APIServerConfig.ContainerImage == image || configImage == image {
				return errUpdateSkipped
			}

			logUpdate(config.ClusterConfig.APIServerConfig.ContainerImage)

			if options.DryRun {
				return errUpdateSkipped
			}

			config.ClusterConfig.APIServerConfig.ContainerImage = image
		case kubeControllerManager:
			if config.ClusterConfig.ControllerManagerConfig == nil {
				config.ClusterConfig.ControllerManagerConfig = &v1alpha1config.ControllerManagerConfig{}
			}

			imageName := constants.KubernetesControllerManagerImage

			privateRepo := strings.Split(config.ClusterConfig.ControllerManagerConfig.ContainerImage, ":")[0]
			if privateRepo != constants.KubernetesControllerManagerImage && !options.PrePullImages {
				imageName = privateRepo
			}

			image := fmt.Sprintf("%s:v%s", imageName, options.Path.ToVersion())
			options.Log(" > using controllerManagerImage %s", image)

			if config.ClusterConfig.ControllerManagerConfig.ContainerImage == image || configImage == image {
				return errUpdateSkipped
			}

			logUpdate(config.ClusterConfig.ControllerManagerConfig.ContainerImage)

			if options.DryRun {
				return errUpdateSkipped
			}

			config.ClusterConfig.ControllerManagerConfig.ContainerImage = image
		case kubeScheduler:
			if config.ClusterConfig.SchedulerConfig == nil {
				config.ClusterConfig.SchedulerConfig = &v1alpha1config.SchedulerConfig{}
			}

			imageName := constants.KubernetesSchedulerImage

			privateRepo := strings.Split(config.ClusterConfig.SchedulerConfig.ContainerImage, ":")[0]
			if privateRepo != constants.KubernetesSchedulerImage && !options.PrePullImages {
				imageName = privateRepo
			}

			image := fmt.Sprintf("%s:v%s", imageName, options.Path.ToVersion())
			options.Log(" > using schedulerImage %s", image)

			if config.ClusterConfig.SchedulerConfig.ContainerImage == image || configImage == image {
				return errUpdateSkipped
			}

			logUpdate(config.ClusterConfig.SchedulerConfig.ContainerImage)

			if options.DryRun {
				return errUpdateSkipped
			}

			config.ClusterConfig.SchedulerConfig.ContainerImage = image
		default:
			return fmt.Errorf("unsupported service %q", service)
		}

		return nil
	}
}

func getManifests(ctx context.Context, cluster UpgradeProvider) ([]*unstructured.Unstructured, error) {
	talosclient, err := cluster.Client()
	if err != nil {
		return nil, err
	}

	defer cluster.Close() //nolint:errcheck

	md, _ := metadata.FromOutgoingContext(ctx)
	if nodes := md["nodes"]; len(nodes) > 0 {
		ctx = client.WithNode(ctx, nodes[0])
	}

	return manifests.GetBootstrapManifests(ctx, talosclient.COSI, nil)
}

func syncManifests(ctx context.Context, objects []*unstructured.Unstructured, cluster UpgradeProvider, options UpgradeOptions) error {
	config, err := cluster.K8sRestConfig(ctx)
	if err != nil {
		return err
	}

	return manifests.SyncWithLog(ctx, objects, config, options.DryRun, options.Log)
}

//nolint:gocyclo
func checkPodStatus(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions, service, node, configVersion string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	defer k8sClient.Close() //nolint:errcheck

	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		k8sClient, 10*time.Second,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.LabelSelector = fmt.Sprintf("k8s-app = %s", service)
		}),
	)

	notifyCh := make(chan *v1.Pod)

	informer := informerFactory.Core().V1().Pods().Informer()

	if err := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		options.Log("kubernetes endpoint watch error: %s", err)
	}); err != nil {
		return fmt.Errorf("error setting watch error handler: %w", err)
	}

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { channel.SendWithContext(ctx, notifyCh, obj.(*v1.Pod)) },
		DeleteFunc: func(_ interface{}) {},
		UpdateFunc: func(_, obj interface{}) { channel.SendWithContext(ctx, notifyCh, obj.(*v1.Pod)) },
	}); err != nil {
		return fmt.Errorf("error adding watch event handler: %w", err)
	}

	informerFactory.Start(ctx.Done())

	defer func() {
		cancel()
		informerFactory.Shutdown()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case pod := <-notifyCh:
			if pod.Status.HostIP != node {
				continue
			}

			if pod.Annotations[constants.AnnotationStaticPodConfigVersion] != configVersion {
				options.Log(" > %q: %s: waiting, config version mismatch: got %q, expected %q", node, service, pod.Annotations[constants.AnnotationStaticPodConfigVersion], configVersion)

				continue
			}

			ready := false

			for _, condition := range pod.Status.Conditions {
				if condition.Type != v1.PodReady {
					continue
				}

				if condition.Status == v1.ConditionTrue {
					ready = true

					break
				}
			}

			if !ready {
				options.Log(" > %q: %s: pod is not ready, waiting", node, service)

				continue
			}

			return nil
		}
	}
}
