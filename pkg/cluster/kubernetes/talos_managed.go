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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/go-cmp/cmp"
	"github.com/talos-systems/go-retry/retry"
	"google.golang.org/grpc/codes"
	"gopkg.in/yaml.v3"
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
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// UpgradeProvider are the cluster interfaces required by upgrade process.
type UpgradeProvider interface {
	cluster.ClientProvider
	cluster.K8sProvider
}

var deprecations = map[string][]string{
	// https://kubernetes.io/blog/2021/07/14/upcoming-changes-in-kubernetes-1-22/#api-changes
	"1.21->1.22": {
		"validatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
		"mutatingwebhookconfigurations.v1beta1.admissionregistration.k8s.io",
		"customresourcedefinitions.v1beta1.apiextensions.k8s.io",
		"apiservices.v1beta1.apiregistration.k8s.io",
		"leases.v1beta1.coordination.k8s.io",
		"ingresses.v1beta1.extensions",
		"ingresses.v1beta1.networking.k8s.io",
	},
}

// UpgradeTalosManaged the Kubernetes control plane.
//
//nolint:gocyclo,cyclop
func UpgradeTalosManaged(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions) error {
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

	options.masterNodes, err = k8sClient.NodeIPs(ctx, machinetype.TypeControlPlane)
	if err != nil {
		return fmt.Errorf("error fetching master nodes: %w", err)
	}

	if len(options.masterNodes) == 0 {
		return fmt.Errorf("no master nodes discovered")
	}

	options.Log("discovered master nodes %q", options.masterNodes)

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

	for _, node := range options.masterNodes {
		if err := upgradeStaticPodOnNode(ctx, cluster, options, service, node); err != nil {
			return fmt.Errorf("error updating node %q: %w", node, err)
		}
	}

	return nil
}

//nolint:gocyclo
func upgradeStaticPodOnNode(ctx context.Context, cluster UpgradeProvider, options UpgradeOptions, service, node string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNodes(ctx, node)

	options.Log(" > %q: starting update", node)

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

	err = patchNodeConfig(ctx, cluster, node, upgradeStaticPodPatcher(options, service, watchInitial.Resource))
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
	options.Log(" > %q: waiting for API server state pod update", node)

	var expectedConfigVersion string

	if !skipConfigWait {
		var watchUpdated client.WatchResponse

		watchUpdated, err = watchClient.Recv()
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

		configData := configResource.(*resource.Any).Value().(map[string]interface{}) //nolint:errcheck,forcetypeassert
		configImage := configData["image"].(string)                                   //nolint:errcheck,forcetypeassert

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

	listClient, err := talosclient.Resources.List(ctx, k8s.ControlPlaneNamespaceName, k8s.ManifestType)
	if err != nil {
		return nil, err
	}

	objects := []*unstructured.Unstructured{}

	for {
		msg, err := listClient.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				return objects, nil
			}

			return nil, err
		}

		if msg.Metadata.GetError() != "" {
			return nil, fmt.Errorf(msg.Metadata.GetError())
		}

		if msg.Resource == nil {
			continue
		}

		// TODO: fix that when we get resource API to work through protobufs
		out, err := resource.MarshalYAML(msg.Resource)
		if err != nil {
			return nil, err
		}

		data, err := yaml.Marshal(out)
		if err != nil {
			return nil, err
		}

		manifest := struct {
			Objects []map[string]interface{} `yaml:"spec"`
		}{}

		if err = yaml.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}

		for _, o := range manifest.Objects {
			obj := &unstructured.Unstructured{Object: o}

			// kubeproxy daemon set is updated as part of a different flow
			if obj.GetName() == kubeProxy && obj.GetKind() == "DaemonSet" {
				continue
			}

			objects = append(objects, obj)
		}
	}
}

//nolint:gocyclo,cyclop
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

	var (
		resp    *unstructured.Unstructured
		mapping *meta.RESTMapping
		// list of deployments to wait for to become ready after update
		deployments []*unstructured.Unstructured
	)

	options.Log("updating manifests")

	for _, obj := range objects {
		mapping, err = mapper.RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
		if err != nil {
			return fmt.Errorf("error creating mapping for object %s: %w", obj.GetName(), err)
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// namespaced resources should specify the namespace
			dr = k8sClient.Resource(mapping.Resource).Namespace(obj.GetNamespace())
		} else {
			// for cluster-wide resources
			dr = k8sClient.Resource(mapping.Resource)
		}

		var diff string

		exists := true

		diff, err = getResourceDiff(ctx, dr, obj)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}

			exists = false
			diff = "resource is going to be created"
		}

		options.Log(" > apply manifest %s %s", obj.GetKind(), obj.GetName())

		switch {
		case options.DryRun:
			var diffInfo string
			if diff != "" {
				diffInfo = fmt.Sprintf(", diff:\n%s", diff)
			}

			options.Log(" > apply skipped in dry run%s", diffInfo)

			continue
		case !exists:
			resp, err = dr.Create(ctx, obj, metav1.CreateOptions{})
		case diff != "":
			resp, err = dr.Update(ctx, obj, metav1.UpdateOptions{})
		default:
			options.Log(" > apply skipped: nothing to update")

			continue
		}

		if err != nil {
			return err
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

			namespaceNames := make([]string, 0, len(namespaces))

			for _, ns := range namespaces {
				namespaceNames = append(namespaceNames, ns.Name)
			}

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
