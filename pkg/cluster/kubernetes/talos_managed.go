// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/go-cmp/cmp"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-retry/retry"
	"google.golang.org/grpc/metadata"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/client"
	v1alpha1config "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	machinetype "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// UpgradeProvider are the cluster interfaces required by upgrade process.
type UpgradeProvider interface {
	cluster.ClientProvider
	cluster.K8sProvider
}

var deprecations = map[string][]string{
	// https://kubernetes.io/docs/reference/using-api/deprecation-guide/
	"1.21->1.22": {
		"validatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
		"mutatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
		"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
		"apiservices.v1beta1.apiregistration.k8s.io",
		"leases.v1beta1.coordination.k8s.io",
		"ingresses.v1beta1.extensions",
		"ingresses.v1beta1.networking.k8s.io",
	},
	"1.24->1.25": {
		"cronjobs.v1beta1.batch",
		"endpointslices.v1beta1.discovery.k8s.io",
		"events.v1beta1.events.k8s.io",
		"horizontalpodautoscalers.v2beta1.autoscaling",
		"poddisruptionbudgets.v1beta1.policy",
		"podsecuritypolicies.v1beta1.policy",
		"runtimeclasses.v1beta1.node.k8s.io",
	},
	"1.25->1.26": {
		"flowschemas.v1beta1.flowcontrol.apiserver.k8s.io",
		"prioritylevelconfigurations.v1beta1.flowcontrol.apiserver.k8s.io",
		"horizontalpodautoscalers.v2beta2.autoscaling",
	},
}

// UpgradeTalosManaged the Kubernetes control plane.
//
//nolint:gocyclo,cyclop
func UpgradeTalosManaged(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) error {
	// strip leading `v` from Kubernetes version
	options.FromVersion = strings.TrimLeft(options.FromVersion, "v")
	options.ToVersion = strings.TrimLeft(options.ToVersion, "v")

	switch path := options.Path(); path {
	// nothing for all those
	case "1.19->1.19":
	case "1.19->1.20":
	case "1.20->1.20":
	case "1.20->1.21":
	case "1.21->1.21":
	case "1.21->1.22":
	case "1.22->1.22":
	case "1.22->1.23":
	case "1.23->1.23":
	case "1.23->1.24":
	case "1.24->1.24":
	case "1.24->1.25":
	case "1.25->1.25":
	case "1.25->1.26":
	case "1.26->1.26":

	default:
		return fmt.Errorf("unsupported upgrade path %q (from %q to %q)", path, options.FromVersion, options.ToVersion)
	}

	if err := checkDeprecated(ctx, cluster, options); err != nil {
		return err
	}

	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

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

	for _, service := range []string{kubeAPIServer, kubeControllerManager, kubeScheduler} {
		if err = upgradeStaticPod(ctx, cluster, options, service); err != nil {
			return fmt.Errorf("failed updating service %q: %w", service, err)
		}
	}

	if err = upgradeDaemonset(ctx, k8sClient.Clientset, kubeProxy, options); err != nil {
		if apierrors.IsNotFound(err) {
			options.Log("kube-proxy skipped as DaemonSet was not found")
		} else {
			return fmt.Errorf("error updating kube-proxy: %w", err)
		}
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

func upgradeStaticPod(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions, service string) error {
	options.Log("updating %q to version %q", service, options.ToVersion)

	for _, node := range options.controlPlaneNodes {
		if err := upgradeStaticPodOnNode(ctx, cluster, options, service, node); err != nil {
			return fmt.Errorf("error updating node %q: %w", node, err)
		}
	}

	return nil
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

	err = patchNodeConfig(ctx, cluster, node, upgradeStaticPodPatcher(options, service, initialConfig))
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

	if err = retry.Constant(3*time.Minute, retry.WithUnits(10*time.Second)).Retry(func() error {
		return checkPodStatus(ctx, cluster, service, node, expectedConfigVersion)
	}); err != nil {
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
			version := options.FromVersion

			if oldImage == "" {
				version = options.FromVersion
			}

			if len(parts) > 1 {
				version = parts[1]
			}

			options.Log(" > update %s: %s -> %s", service, version, options.ToVersion)

			if options.DryRun {
				options.Log(" > skipped in dry-run")
			}
		}

		switch service {
		case kubeAPIServer:
			if config.ClusterConfig.APIServerConfig == nil {
				config.ClusterConfig.APIServerConfig = &v1alpha1config.APIServerConfig{}
			}

			image := fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, options.ToVersion)

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

			image := fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, options.ToVersion)

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

			image := fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, options.ToVersion)

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

//nolint:gocyclo
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

	items, err := safe.StateList[*k8s.Manifest](ctx, talosclient.COSI, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
	if err != nil {
		return nil, err
	}

	it := safe.IteratorFromList(items)

	objects := []*unstructured.Unstructured{}

	for it.Next() {
		for _, o := range it.Value().TypedSpec().Items {
			obj := &unstructured.Unstructured{Object: o.Object}

			// kubeproxy daemon set is updated as part of a different flow
			if obj.GetName() == kubeProxy && obj.GetKind() == "DaemonSet" {
				continue
			}

			objects = append(objects, obj)
		}
	}

	return objects, nil
}

func updateManifest(
	ctx context.Context,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	k8sClient dynamic.Interface,
	obj *unstructured.Unstructured,
	dryRun bool,
) (
	resp *unstructured.Unstructured,
	diff string,
	skipped bool,
	err error,
) {
	mapping, err := mapper.RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
	if err != nil {
		err = fmt.Errorf("error creating mapping for object %s: %w", obj.GetName(), err)

		return nil, "", false, err
	}

	var dr dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		// namespaced resources should specify the namespace
		dr = k8sClient.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		// for cluster-wide resources
		dr = k8sClient.Resource(mapping.Resource)
	}

	exists := true

	diff, err = getResourceDiff(ctx, dr, obj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, "", false, err
		}

		exists = false
		diff = "resource is going to be created"
	}

	switch {
	case dryRun:
		return nil, diff, exists, nil
	case !exists:
		resp, err = dr.Create(ctx, obj, metav1.CreateOptions{})
	case diff != "":
		resp, err = dr.Update(ctx, obj, metav1.UpdateOptions{})
	default:
		skipped = true
	}

	return resp, diff, skipped, err
}

//nolint:gocyclo
func syncManifests(ctx context.Context, objects []*unstructured.Unstructured, cluster UpgradeProvider, options UpgradeOptions) error {
	config, err := cluster.K8sRestConfig(ctx)
	if err != nil {
		return err
	}

	dialer := kubernetes.NewDialer()
	config.Dial = dialer.DialContext

	defer dialer.CloseAll()

	k8sClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

	// list of deployments to wait for to become ready after update
	var deployments []*unstructured.Unstructured

	options.Log("updating manifests")

	for _, obj := range objects {
		options.Log(" > processing manifest %s %s", obj.GetKind(), obj.GetName())

		var (
			resp    *unstructured.Unstructured
			diff    string
			skipped bool
		)

		err = retry.Constant(3*time.Minute, retry.WithUnits(10*time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx, func(ctx context.Context) error {
			resp, diff, skipped, err = updateManifest(ctx, mapper, k8sClient, obj, options.DryRun)
			if kubernetes.IsRetryableError(err) || apierrors.IsConflict(err) {
				return retry.ExpectedError(err)
			}

			return err
		})

		if err != nil {
			return err
		}

		switch {
		case options.DryRun:
			var diffInfo string
			if diff != "" {
				diffInfo = fmt.Sprintf(", diff:\n%s", diff)
			}

			options.Log(" < apply skipped in dry run%s", diffInfo)

			continue
		case skipped:
			options.Log(" < apply skipped: nothing to update")

			continue
		}

		if resp.GetKind() == "Deployment" {
			deployments = append(deployments, resp)
		}

		options.Log(" < update applied, diff:\n%s", diff)
	}

	if len(deployments) == 0 {
		return nil
	}

	clientset, err := cluster.K8sHelper(ctx)
	if err != nil {
		return err
	}

	defer clientset.Close() //nolint:errcheck

	for _, obj := range deployments {
		obj := obj

		err := retry.Constant(3*time.Minute, retry.WithUnits(10*time.Second)).Retry(func() error {
			deployment, err := clientset.AppsV1().Deployments(obj.GetNamespace()).Get(ctx, obj.GetName(), metav1.GetOptions{})
			if err != nil {
				return err
			}

			if deployment.Status.ReadyReplicas != deployment.Status.Replicas || deployment.Status.UpdatedReplicas != deployment.Status.Replicas {
				return retry.ExpectedErrorf("deployment %s ready replicas %d != replicas %d", deployment.Name, deployment.Status.ReadyReplicas, deployment.Status.Replicas)
			}

			options.Log(" > updated %s", deployment.GetName())

			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func getResourceDiff(ctx context.Context, dr dynamic.ResourceInterface, obj *unstructured.Unstructured) (string, error) {
	current, err := dr.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	obj.SetResourceVersion(current.GetResourceVersion())

	resp, err := dr.Update(ctx, obj, metav1.UpdateOptions{
		DryRun: []string{"All"},
	})
	if err != nil {
		return "", err
	}

	ignoreKey := func(key string) {
		delete(current.Object, key)
		delete(resp.Object, key)
	}

	ignoreKey("metadata") // contains lots of dynamic data generated by kubernetes

	if resp.GetKind() == "ServiceAccount" {
		ignoreKey("secrets") // injected by Kubernetes in ServiceAccount objects
	}

	x, err := k8syaml.Marshal(current)
	if err != nil {
		return "", err
	}

	y, err := k8syaml.Marshal(resp)
	if err != nil {
		return "", err
	}

	return cmp.Diff(string(x), string(y)), nil
}

//nolint:gocyclo
func checkPodStatus(ctx context.Context, cluster UpgradeProvider, service, node, configVersion string) error {
	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("k8s-app = %s", service),
	})
	if err != nil {
		if kubernetes.IsRetryableError(err) {
			return retry.ExpectedError(err)
		}

		return err
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
			if condition.Type != v1.PodReady {
				continue
			}

			if condition.Status == v1.ConditionTrue {
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

//nolint:gocyclo,cyclop
func checkDeprecated(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) error {
	options.Log("checking for resource APIs to be deprecated in version %s", options.ToVersion)

	config, err := cluster.K8sRestConfig(ctx)
	if err != nil {
		return err
	}

	config.WarningHandler = rest.NewWarningWriter(io.Discard, rest.WarningWriterOptions{})

	k8sClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	staticClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %s", err)
	}

	hasDeprecated := false

	warnings := bytes.NewBuffer([]byte{})

	w := tabwriter.NewWriter(warnings, 0, 0, 3, ' ', 0)

	resources, ok := deprecations[options.Path()]
	if !ok {
		return nil
	}

	var namespaces *v1.NamespaceList

	namespaces, err = staticClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	serverResources, err := dc.ServerPreferredNamespacedResources()
	if err != nil {
		return err
	}

	namespacedResources := map[string]struct{}{}

	for _, list := range serverResources {
		for _, resource := range list.APIResources {
			namespacedResources[resource.Name] = struct{}{}
		}
	}

	for _, resource := range resources {
		gvr, _ := schema.ParseResourceArg(resource)

		if gvr == nil {
			return fmt.Errorf("failed to parse group version resource %s", resource)
		}

		var res *unstructured.UnstructuredList

		count := 0

		probeResources := func(namespaces ...v1.Namespace) error {
			r := k8sClient.Resource(*gvr)

			namespaceNames := slices.Map(namespaces, func(ns v1.Namespace) string { return ns.Name })

			if len(namespaceNames) == 0 {
				namespaceNames = append(namespaceNames, "default")
			}

			for _, ns := range namespaceNames {
				if ns != "default" {
					r.Namespace(ns)
				}

				res, err = r.List(ctx, metav1.ListOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						return nil
					}

					return err
				}

				count += len(res.Items)
			}

			return nil
		}

		checkNamespaces := []v1.Namespace{}

		if _, ok := namespacedResources[gvr.Resource]; ok {
			checkNamespaces = namespaces.Items
		}

		if err = probeResources(checkNamespaces...); err != nil {
			return err
		}

		if count > 0 {
			if !hasDeprecated {
				fmt.Fprintf(w, "RESOURCE\tCOUNT\n")
			}

			hasDeprecated = true

			fmt.Fprintf(w, "%s\t%d\n", resource, len(res.Items))
		}
	}

	if hasDeprecated {
		if err = w.Flush(); err != nil {
			return err
		}

		options.Log("WARNING: found resources which are going to be deprecated/migrated in the version %s", options.ToVersion)
		options.Log(warnings.String())
	}

	return nil
}
