// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/talos-systems/go-retry/retry"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"

	"github.com/talos-systems/talos/pkg/cluster"
	k8s "github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// UpgradeSelfHosted the Kubernetes control plane.
func UpgradeSelfHosted(ctx context.Context, cluster cluster.K8sProvider, options UpgradeOptions) error {
	switch path := options.Path(); path {
	case "1.18->1.19":
		fallthrough
	case "1.19->1.19":
		return hyperkubeUpgrade(ctx, cluster, options)

	case "1.19->1.20":
		options.extraUpdaters = append(options.extraUpdaters, addControlPlaneToleration())
		options.podCheckpointerExtraUpdaters = append(options.podCheckpointerExtraUpdaters, addControlPlaneToleration())

		serviceAccountUpdater, err := kubeAPIServerServiceAccountPatch(options)
		if err != nil {
			return err
		}

		options.extraUpdaters = append(options.extraUpdaters, serviceAccountUpdater)

		if err = serviceAccountSecretsUpdate(ctx, cluster); err != nil {
			return err
		}

		return hyperkubeUpgrade(ctx, cluster, options)

	case "1.20->1.20":
		fallthrough
	case "1.20->1.21":
		fallthrough
	case "1.21->1.21":
		return hyperkubeUpgrade(ctx, cluster, options)

	default:
		return fmt.Errorf("unsupported upgrade path %q (from %q to %q)", path, options.FromVersion, options.ToVersion)
	}
}

// hyperkubeUpgrade upgrades from hyperkube-based to distroless images in 1.19.
func hyperkubeUpgrade(ctx context.Context, cluster cluster.K8sProvider, options UpgradeOptions) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return fmt.Errorf("error building K8s client: %w", err)
	}

	if err = podCheckpointerGracePeriod(ctx, clientset, "0m"); err != nil {
		return fmt.Errorf("error setting pod-checkpointer grace period: %w", err)
	}

	graceTimeout := 5 * time.Minute

	fmt.Printf("sleeping %s to let the pod-checkpointer self-checkpoint be updated\n", graceTimeout.String())
	time.Sleep(graceTimeout)

	daemonsets := []string{kubeAPIServer, kubeControllerManager, kubeScheduler, kubeProxy}

	for _, ds := range daemonsets {
		if err = hyperkubeUpgradeDs(ctx, clientset, ds, options); err != nil {
			return fmt.Errorf("failed updating daemonset %q: %w", ds, err)
		}
	}

	if err = podCheckpointerGracePeriod(ctx, clientset, graceTimeout.String(), options.podCheckpointerExtraUpdaters...); err != nil {
		return fmt.Errorf("error setting pod-checkpointer grace period: %w", err)
	}

	return nil
}

//nolint:gocyclo
func updateDaemonset(ctx context.Context, clientset *kubernetes.Clientset, ds string, updateFunc func(daemonset *appsv1.DaemonSet) error) error {
	daemonset, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, ds, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error fetching daemonset: %w", err)
	}

	oldData, err := json.Marshal(daemonset)
	if err != nil {
		return fmt.Errorf("error marshaling deployment: %w", err)
	}

	if err = updateFunc(daemonset); err != nil {
		return err
	}

	newData, err := json.Marshal(daemonset)
	if err != nil {
		return fmt.Errorf("error marshaling new deployment: %w", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, appsv1.DaemonSet{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch: %w", err)
	}

	_, err = clientset.AppsV1().DaemonSets(namespace).Patch(ctx, daemonset.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{
		FieldManager: "talos",
	})
	if err != nil {
		return fmt.Errorf("error patching deployment: %w", err)
	}

	// give k8s some time
	time.Sleep(10 * time.Second)

	return retry.Constant(5*time.Minute, retry.WithUnits(10*time.Second)).Retry(func() error {
		daemonset, err = clientset.AppsV1().DaemonSets(namespace).Get(ctx, ds, metav1.GetOptions{})
		if err != nil {
			if k8s.IsRetryableError(err) {
				return retry.ExpectedError(err)
			}

			return retry.UnexpectedError(fmt.Errorf("error fetching daemonset: %w", err))
		}

		if daemonset.Status.UpdatedNumberScheduled != daemonset.Status.DesiredNumberScheduled {
			return retry.ExpectedError(fmt.Errorf("expected current number up-to-date for %s to be %d, got %d", ds, daemonset.Status.UpdatedNumberScheduled, daemonset.Status.CurrentNumberScheduled))
		}

		if daemonset.Status.CurrentNumberScheduled != daemonset.Status.DesiredNumberScheduled {
			return retry.ExpectedError(fmt.Errorf("expected current number scheduled for %s to be %d, got %d", ds, daemonset.Status.DesiredNumberScheduled, daemonset.Status.CurrentNumberScheduled))
		}

		if daemonset.Status.NumberAvailable != daemonset.Status.DesiredNumberScheduled {
			return retry.ExpectedError(fmt.Errorf("expected number available for %s to be %d, got %d", ds, daemonset.Status.DesiredNumberScheduled, daemonset.Status.NumberAvailable))
		}

		if daemonset.Status.NumberReady != daemonset.Status.DesiredNumberScheduled {
			return retry.ExpectedError(fmt.Errorf("expected number ready for %s to be %d, got %d", ds, daemonset.Status.DesiredNumberScheduled, daemonset.Status.NumberReady))
		}

		return nil
	})
}

func podCheckpointerGracePeriod(ctx context.Context, clientset *kubernetes.Clientset, gracePeriod string, extraUpdaters ...daemonsetUpdater) error {
	fmt.Printf("updating pod-checkpointer grace period to %q\n", gracePeriod)

	return updateDaemonset(ctx, clientset, "pod-checkpointer", func(daemonset *appsv1.DaemonSet) error {
		if len(daemonset.Spec.Template.Spec.Containers) != 1 {
			return fmt.Errorf("unexpected number of containers: %d", len(daemonset.Spec.Template.Spec.Containers))
		}

		args := daemonset.Spec.Template.Spec.Containers[0].Command
		for i := range args {
			if strings.HasPrefix(args[i], "--checkpoint-grace-period=") {
				args[i] = fmt.Sprintf("--checkpoint-grace-period=%s", gracePeriod)
			}
		}

		for _, updater := range extraUpdaters {
			if err := updater("pod-checkpointer", daemonset); err != nil {
				return err
			}
		}

		return nil
	})
}

//nolint:gocyclo
func hyperkubeUpgradeDs(ctx context.Context, clientset *kubernetes.Clientset, ds string, options UpgradeOptions) error {
	if ds == kubeAPIServer {
		fmt.Printf("temporarily taking %q out of pod-checkpointer control\n", ds)

		if err := updateDaemonset(ctx, clientset, ds, func(daemonset *appsv1.DaemonSet) error {
			delete(daemonset.Spec.Template.Annotations, checkpointerAnnotation)

			return nil
		}); err != nil {
			return err
		}
	}

	fmt.Printf("updating daemonset %q to version %q\n", ds, options.ToVersion)

	return updateDaemonset(ctx, clientset, ds, func(daemonset *appsv1.DaemonSet) error {
		if len(daemonset.Spec.Template.Spec.Containers) != 1 {
			return fmt.Errorf("unexpected number of containers: %d", len(daemonset.Spec.Template.Spec.Containers))
		}

		args := daemonset.Spec.Template.Spec.Containers[0].Command
		if args[0] == "./hyperkube" || args[0] == "/hyperkube" {
			args[0] = "/go-runner"
			args[1] = fmt.Sprintf("/usr/local/bin/%s", ds)

			if ds == kubeProxy {
				daemonset.Spec.Template.Spec.Containers[0].Command = daemonset.Spec.Template.Spec.Containers[0].Command[1:]
			}
		}

		switch ds {
		case kubeAPIServer:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, options.ToVersion)
		case kubeControllerManager:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, options.ToVersion)
		case kubeScheduler:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, options.ToVersion)
		case kubeProxy:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:v%s", constants.KubernetesProxyImage, options.ToVersion)
		default:
			return fmt.Errorf("failed to build new image spec")
		}

		if ds == kubeAPIServer {
			if daemonset.Spec.Template.Annotations == nil {
				daemonset.Spec.Template.Annotations = make(map[string]string)
			}

			daemonset.Spec.Template.Annotations[checkpointerAnnotation] = "true"
		}

		for _, updater := range options.extraUpdaters {
			if err := updater(ds, daemonset); err != nil {
				return err
			}
		}

		return nil
	})
}

func serviceAccountSecretsUpdate(ctx context.Context, cluster cluster.K8sProvider) error {
	const serviceAccountKey = "service-account.key"

	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return fmt.Errorf("error building K8s client: %w", err)
	}

	apiServerSecrets, err := clientset.CoreV1().Secrets(namespace).Get(ctx, kubeAPIServer, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error fetching kube-apiserver secrets: %w", err)
	}

	if _, ok := apiServerSecrets.Data[serviceAccountKey]; ok {
		return nil
	}

	controllerManagerSecrets, err := clientset.CoreV1().Secrets(namespace).Get(ctx, kubeControllerManager, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error fetching kube-controller-manager secrets: %w", err)
	}

	if _, ok := controllerManagerSecrets.Data[serviceAccountKey]; !ok {
		return fmt.Errorf("kube-controller-manager secrets missing %q secret", serviceAccountKey)
	}

	apiServerSecrets.Data[serviceAccountKey] = controllerManagerSecrets.Data[serviceAccountKey]

	_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, apiServerSecrets, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating kube-apiserver secrets: %w", err)
	}

	fmt.Printf("patched kube-apiserver secrets for %q\n", serviceAccountKey)

	return nil
}

func addControlPlaneToleration() daemonsetUpdater {
	return func(ds string, daemonset *appsv1.DaemonSet) error {
		if ds == kubeProxy {
			return nil
		}

		tolerationFound := false

		for _, toleration := range daemonset.Spec.Template.Spec.Tolerations {
			if toleration.Key == constants.LabelNodeRoleControlPlane {
				tolerationFound = true

				break
			}
		}

		if tolerationFound {
			return nil
		}

		daemonset.Spec.Template.Spec.Tolerations = append(daemonset.Spec.Template.Spec.Tolerations, corev1.Toleration{
			Key:      constants.LabelNodeRoleControlPlane,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})

		return nil
	}
}

func kubeAPIServerServiceAccountPatch(options UpgradeOptions) (daemonsetUpdater, error) {
	if options.ControlPlaneEndpoint == "" {
		return nil, fmt.Errorf("control plane endpoint is required for service account patch")
	}

	return func(ds string, daemonset *appsv1.DaemonSet) error {
		if ds != kubeAPIServer {
			return nil
		}

		argExists := func(argName string) bool {
			prefix := fmt.Sprintf("--%s=", argName)

			for _, arg := range daemonset.Spec.Template.Spec.Containers[0].Command {
				if strings.HasPrefix(arg, prefix) {
					return true
				}
			}

			return false
		}

		if !argExists("api-audiences") {
			daemonset.Spec.Template.Spec.Containers[0].Command = append(daemonset.Spec.Template.Spec.Containers[0].Command,
				fmt.Sprintf("--api-audiences=%s", options.ControlPlaneEndpoint))
		}

		if !argExists("service-account-issuer") {
			daemonset.Spec.Template.Spec.Containers[0].Command = append(daemonset.Spec.Template.Spec.Containers[0].Command,
				fmt.Sprintf("--service-account-issuer=%s", options.ControlPlaneEndpoint))
		}

		if !argExists("service-account-signing-key") {
			daemonset.Spec.Template.Spec.Containers[0].Command = append(daemonset.Spec.Template.Spec.Containers[0].Command,
				"--service-account-signing-key-file=/etc/kubernetes/secrets/service-account.key")
		}

		return nil
	}, nil
}
