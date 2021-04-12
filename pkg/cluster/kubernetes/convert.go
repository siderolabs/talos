// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/go-retry/retry"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	v1alpha1config "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	machinetype "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/k8s"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// ConvertOptions are options for convert tasks.
type ConvertOptions struct {
	ControlPlaneEndpoint     string
	ForceYes                 bool
	OnlyRemoveInitializedKey bool

	Node string

	masterNodes []string
}

// ConvertProvider are the cluster interfaces required by converter.
type ConvertProvider interface {
	cluster.ClientProvider
	cluster.K8sProvider
}

// ConvertToStaticPods the self-hosted Kubernetes control plane to Talos-managed static pods-based control plane.
//
//nolint:gocyclo,cyclop
func ConvertToStaticPods(ctx context.Context, cluster ConvertProvider, options ConvertOptions) error {
	// only used in manual conversion process
	if options.OnlyRemoveInitializedKey {
		return removeInitializedKey(ctx, cluster, options.Node)
	}

	var err error

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

	selfHosted, err := IsSelfHostedControlPlane(ctx, cluster, options.masterNodes[0])
	if err != nil {
		return fmt.Errorf("error checkign self-hosted control plane status: %w", err)
	}

	fmt.Printf("current self-hosted status: %v\n", selfHosted)

	if selfHosted {
		if err = updateNodeConfig(ctx, cluster, &options); err != nil {
			return err
		}

		if err = waitResourcesReady(ctx, cluster, &options); err != nil {
			return err
		}

		fmt.Println("Talos generated control plane static pod definitions and bootstrap manifests, please verify them with commands:")
		fmt.Printf("\ttalosctl -n <master node IP> get %s\n", k8s.StaticPodType)
		fmt.Printf("\ttalosctl -n <master node IP> get %s\n", k8s.ManifestType)
		fmt.Println()
		fmt.Println("in order to remove self-hosted control plane, pod-checkpointer component needs to be disabled")
		fmt.Println("once pod-checkpointer is disabled, the cluster shouldn't be rebooted until the entire conversion process is complete")

		if !options.ForceYes {
			var yes bool

			yes, err = askYesNo("confirm disabling pod-checkpointer to proceed with control plane update")
			if err != nil {
				return err
			}

			if !yes {
				return fmt.Errorf("aborted")
			}
		}

		if err = disablePodCheckpointer(ctx, cluster); err != nil {
			return err
		}

		if !options.ForceYes {
			var yes bool

			yes, err = askYesNo("confirm applying static pod definitions and manifests")
			if err != nil {
				return err
			}

			if !yes {
				return fmt.Errorf("aborted")
			}
		}

		if err = removeInitializedKey(ctx, cluster, options.masterNodes[0]); err != nil {
			return err
		}
	}

	for _, ds := range []string{kubeAPIServer, kubeControllerManager, kubeScheduler} {
		// API server won't be ready as it can't bind to the port
		if err = waitForStaticPods(ctx, cluster, &options, ds, ds != kubeAPIServer); err != nil {
			return err
		}
	}

	for _, ds := range []string{kubeAPIServer, kubeControllerManager, kubeScheduler} {
		if err = deleteDaemonset(ctx, cluster, ds, false); err != nil {
			return err
		}

		if err = waitForStaticPods(ctx, cluster, &options, ds, true); err != nil {
			return err
		}
	}

	fmt.Println("conversion process completed successfully")

	return nil
}

// IsSelfHostedControlPlane returns true if cluster is still running bootkube self-hosted control plane.
func IsSelfHostedControlPlane(ctx context.Context, cluster cluster.ClientProvider, node string) (bool, error) {
	c, err := cluster.Client()
	if err != nil {
		return false, fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNodes(ctx, node)

	resources, err := c.Resources.Get(ctx, v1alpha1.NamespaceName, v1alpha1.BootstrapStatusType, v1alpha1.BootstrapStatusID)
	if err != nil {
		return false, fmt.Errorf("error fetching bootstrapStatus resource: %w", err)
	}

	if len(resources) != 1 {
		return false, fmt.Errorf("expected 1 instance of bootstrapStatus resource, got %d", len(resources))
	}

	r := resources[0]

	return r.Resource.(*resource.Any).Value().(map[string]interface{})["selfHostedControlPlane"].(bool), nil
}

// updateNodeConfig reads self-hosted settings and secrets from K8s and stores them back to node configs.
//
//nolint:gocyclo
func updateNodeConfig(ctx context.Context, cluster ConvertProvider, options *ConvertOptions) error {
	fmt.Println("gathering control plane configuration")

	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	type NodeConfigPath struct {
		ServiceAccount *x509.PEMEncodedKey
		AggregatorCA   *x509.PEMEncodedCertificateAndKey

		KubeAPIServerImage         string
		KubeControllerManagerImage string
		KubeSchedulerImage         string
	}

	var patch NodeConfigPath

	secret, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, kubeControllerManager, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error fetching kube-controller-manager secret: %w", err)
	}

	patch.ServiceAccount = &x509.PEMEncodedKey{}

	patch.ServiceAccount.Key = secret.Data["service-account.key"]
	if patch.ServiceAccount.Key == nil {
		return fmt.Errorf("service-account.key missing")
	}

	fmt.Println("aggregator CA key can't be recovered from bootkube-boostrapped control plane, generating new CA")

	aggregatorCA, err := generate.NewAggregatorCA(time.Now())
	if err != nil {
		return fmt.Errorf("error generating aggregator CA: %w", err)
	}

	patch.AggregatorCA = x509.NewCertificateAndKeyFromCertificateAuthority(aggregatorCA)

	for _, name := range []string{kubeAPIServer, kubeControllerManager, kubeScheduler} {
		var ds *appsv1.DaemonSet

		ds, err = k8sClient.AppsV1().DaemonSets(namespace).Get(ctx, name, v1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error fetching %q daemonset: %w", name, err)
		}

		image := ds.Spec.Template.Spec.Containers[0].Image

		switch name {
		case kubeAPIServer:
			patch.KubeAPIServerImage = image
		case kubeControllerManager:
			patch.KubeControllerManagerImage = image
		case kubeScheduler:
			patch.KubeSchedulerImage = image
		}
	}

	for _, node := range options.masterNodes {
		fmt.Printf("patching master node %q configuration\n", node)

		if err = patchNodeConfig(ctx, cluster, node, func(config *v1alpha1config.Config) error {
			if config.ClusterConfig == nil {
				config.ClusterConfig = &v1alpha1config.ClusterConfig{}
			}

			config.ClusterConfig.ClusterServiceAccount = patch.ServiceAccount
			config.ClusterConfig.ClusterAggregatorCA = patch.AggregatorCA

			if config.ClusterConfig.APIServerConfig == nil {
				config.ClusterConfig.APIServerConfig = &v1alpha1config.APIServerConfig{}
			}

			config.ClusterConfig.APIServerConfig.ContainerImage = patch.KubeAPIServerImage

			if config.ClusterConfig.ControllerManagerConfig == nil {
				config.ClusterConfig.ControllerManagerConfig = &v1alpha1config.ControllerManagerConfig{}
			}

			config.ClusterConfig.ControllerManagerConfig.ContainerImage = patch.KubeControllerManagerImage

			if config.ClusterConfig.SchedulerConfig == nil {
				config.ClusterConfig.SchedulerConfig = &v1alpha1config.SchedulerConfig{}
			}

			config.ClusterConfig.SchedulerConfig.ContainerImage = patch.KubeSchedulerImage

			return nil
		}); err != nil {
			return fmt.Errorf("error patching node %q config: %w", node, err)
		}
	}

	return nil
}

// patchNodeConfig updates node configuration by means of patch function.
//
//nolint:gocyclo
func patchNodeConfig(ctx context.Context, cluster ConvertProvider, node string, patchFunc func(config *v1alpha1config.Config) error) error {
	c, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNodes(ctx, node)

	resources, err := c.Resources.Get(ctx, config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID)
	if err != nil {
		return fmt.Errorf("error fetching config resource: %w", err)
	}

	if len(resources) != 1 {
		return fmt.Errorf("expected 1 instance of config resource, got %d", len(resources))
	}

	r := resources[0]

	yamlConfig, err := yaml.Marshal(r.Resource.Spec())
	if err != nil {
		return fmt.Errorf("error getting YAML config: %w", err)
	}

	config, err := configloader.NewFromBytes(yamlConfig)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	cfg, ok := config.(*v1alpha1config.Config)
	if !ok {
		return fmt.Errorf("config is not v1alpha1 config")
	}

	if !cfg.Persist() {
		return fmt.Errorf("config persistence is disabled, patching is not supported")
	}

	if err = patchFunc(cfg); err != nil {
		return fmt.Errorf("error patching config: %w", err)
	}

	cfgBytes, err := cfg.Bytes()
	if err != nil {
		return fmt.Errorf("error serializing config: %w", err)
	}

	_, err = c.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
		Data:      cfgBytes,
		Immediate: true,
	})
	if err != nil {
		return fmt.Errorf("error applying config: %w", err)
	}

	return nil
}

// waitResourcesReady waits for manifests and static pod definitions to be generated.
//
//nolint:gocyclo
func waitResourcesReady(ctx context.Context, cluster ConvertProvider, options *ConvertOptions) error {
	c, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNodes(ctx, options.masterNodes...)

	fmt.Println("waiting for static pod definitions to be generated")

	if err := retry.Constant(3*time.Minute, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		listClient, err := c.Resources.List(ctx, k8s.ControlPlaneNamespaceName, k8s.StaticPodType)
		if err != nil {
			return retry.UnexpectedError(fmt.Errorf("error listing static pod resources: %w", err))
		}

		count := 0

		for {
			resp, err := listClient.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return retry.UnexpectedError(fmt.Errorf("error list listing static pods resources: %w", err))
			}

			if resp.Resource != nil {
				count++
			}
		}

		if count != len(options.masterNodes)*3 {
			return retry.ExpectedError(fmt.Errorf("expected %d static pods, found %d", len(options.masterNodes)*3, count))
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error waiting for static pods to be generated: %w", err)
	}

	fmt.Println("waiting for manifests to be generated")

	if err := retry.Constant(3*time.Minute, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		listClient, err := c.Resources.List(ctx, k8s.ControlPlaneNamespaceName, k8s.ManifestType)
		if err != nil {
			return retry.UnexpectedError(fmt.Errorf("error listing static pod resources: %w", err))
		}

		nodes := make(map[string]struct{})

		for _, node := range options.masterNodes {
			nodes[node] = struct{}{}
		}

		for {
			resp, err := listClient.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return retry.UnexpectedError(fmt.Errorf("error list listing static pods resources: %w", err))
			}

			if resp.Resource != nil {
				delete(nodes, resp.Metadata.GetHostname())
			}
		}

		if len(nodes) > 0 {
			return retry.ExpectedError(fmt.Errorf("some nodes don't have manifests generated: %v", nodes))
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error waiting for manifests to be generated: %w", err)
	}

	return nil
}

// removeInitializedKey removes bootkube boostrap initialized key releasing static pods and manifests.
func removeInitializedKey(ctx context.Context, cluster cluster.ClientProvider, node string) error {
	c, err := cluster.Client()
	if err != nil {
		return fmt.Errorf("error building Talos API client: %w", err)
	}

	ctx = client.WithNodes(ctx, node)

	fmt.Println("removing self-hosted initialized key")

	_, err = c.MachineClient.RemoveBootkubeInitializedKey(ctx, &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("error removing self-hosted iniitialized key: %w", err)
	}

	return nil
}

// waitForStaticPods waits for static pods to be present in the API server.
//
//nolint:gocyclo
func waitForStaticPods(ctx context.Context, cluster ConvertProvider, options *ConvertOptions, k8sApp string, checkReady bool) error {
	fmt.Printf("waiting for static pods for %q to be present in the API server state\n", k8sApp)

	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	return retry.Constant(3*time.Minute, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{
			LabelSelector: fmt.Sprintf("k8s-app = %s", k8sApp),
		})
		if err != nil {
			if kubernetes.IsRetryableError(err) {
				return retry.ExpectedError(err)
			}

			return retry.UnexpectedError(err)
		}

		count := 0

		for _, pod := range pods.Items {
			if pod.Annotations[constants.AnnotationStaticPodSecretsVersion] == "" {
				continue
			}

			staticPod := false

			for _, ref := range pod.OwnerReferences {
				if ref.Kind == "Node" {
					staticPod = true

					break
				}
			}

			if !staticPod {
				continue
			}

			if checkReady {
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
					continue
				}
			}

			count++
		}

		if count != len(options.masterNodes) {
			return retry.ExpectedError(fmt.Errorf("found only %d static pods for %q, expecting %d pods", count, k8sApp, len(options.masterNodes)))
		}

		return nil
	})
}

// disablePodCheckpointer disables pod checkpointer and takes daemonsets out of pod-checkpointer control.
func disablePodCheckpointer(ctx context.Context, cluster ConvertProvider) error {
	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	fmt.Println("disabling pod-checkpointer")

	if err = deleteDaemonset(ctx, cluster, "pod-checkpointer", false); err != nil {
		return fmt.Errorf("error deleting pod-checkpointer: %w", err)
	}

	// pod-checkpointer should clean up checkpoints after itself
	fmt.Println("checking for active pod checkpoints")

	return retry.Constant(7*time.Minute, retry.WithUnits(20*time.Second), retry.WithErrorLogging(true)).Retry(func() error {
		var checkpoints []string

		checkpoints, err = getActiveCheckpoints(ctx, k8sClient)
		if err != nil {
			if kubernetes.IsRetryableError(err) {
				return retry.ExpectedError(err)
			}

			return retry.UnexpectedError(err)
		}

		if len(checkpoints) > 0 {
			return retry.ExpectedError(fmt.Errorf("found %d active pod checkpoints: %v", len(checkpoints), checkpoints))
		}

		return nil
	})
}

func getActiveCheckpoints(ctx context.Context, k8sClient *kubernetes.Client) ([]string, error) {
	pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %w", err)
	}

	checkpoints := []string{}
	pendingCheckpoints := []string{}

	for _, pod := range pods.Items {
		if _, exists := pod.Annotations[checkpointedPodAnnotation]; exists {
			if pod.Status.Phase == corev1.PodPending {
				pendingCheckpoints = append(pendingCheckpoints, pod.Name)
			}

			checkpoints = append(checkpoints, pod.Name)
		}
	}

	if len(pendingCheckpoints) == len(checkpoints) && len(checkpoints) > 0 {
		log.Printf("deleting pending checkpoints %v", pendingCheckpoints)

		for _, name := range pendingCheckpoints {
			if err = k8sClient.CoreV1().Pods(namespace).Delete(ctx, name, v1.DeleteOptions{
				GracePeriodSeconds: pointer.ToInt64(0),
			}); err != nil {
				return nil, fmt.Errorf("error deleting pod: %w", err)
			}
		}
	}

	return checkpoints, nil
}

// deleteDaemonset deletes daemonset and waits for all the pods to be removed.
//
//nolint:gocyclo
func deleteDaemonset(ctx context.Context, cluster ConvertProvider, k8sApp string, anyPod bool) error {
	fmt.Printf("deleting daemonset %q\n", k8sApp)

	k8sClient, err := cluster.K8sHelper(ctx)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	if err = retry.Constant(time.Minute, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		err = k8sClient.AppsV1().DaemonSets(namespace).Delete(ctx, k8sApp, v1.DeleteOptions{})
		if err != nil {
			if kubernetes.IsRetryableError(err) {
				return retry.ExpectedError(err)
			}

			if apierrors.IsNotFound(err) {
				return nil
			}

			return retry.UnexpectedError(err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error deleting daemonset %q: %w", k8sApp, err)
	}

	return retry.Constant(3*time.Minute, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		pods, err := k8sClient.CoreV1().Pods(namespace).List(ctx, v1.ListOptions{
			LabelSelector: fmt.Sprintf("k8s-app = %s", k8sApp),
		})
		if err != nil {
			if kubernetes.IsRetryableError(err) {
				return retry.ExpectedError(err)
			}

			return retry.UnexpectedError(err)
		}

		count := 0

		for _, pod := range pods.Items {
			if anyPod {
				count++

				continue
			}

			for _, ref := range pod.OwnerReferences {
				if ref.Kind == "DaemonSet" {
					count++
				}
			}
		}

		if count > 0 {
			return retry.ExpectedError(fmt.Errorf("still %d pods found for %q", count, k8sApp))
		}

		return nil
	})
}

func askYesNo(prompt string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [yes/no]: ", prompt)

		response, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		switch strings.ToLower(strings.TrimSpace(response)) {
		case "yes", "y":
			return true, nil
		case "no", "n":
			return false, nil
		}
	}
}
