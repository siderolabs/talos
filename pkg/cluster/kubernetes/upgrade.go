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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const (
	namespace              = "kube-system"
	checkpointerAnnotation = "checkpointer.alpha.coreos.com/checkpoint"

	kubeAPIServer         = "kube-apiserver"
	kubeControllerManager = "kube-controller-manager"
	kubeScheduler         = "kube-scheduler"
	kubeProxy             = "kube-proxy"
)

// Upgrade the Kubernetes control plane.
func Upgrade(ctx context.Context, cluster cluster.K8sProvider, arch, fromVersion, toVersion string) error {
	switch {
	case strings.HasPrefix(fromVersion, "1.18.") && strings.HasPrefix(toVersion, "1.19."):
		return hyperkubeUpgrade(ctx, cluster, arch, toVersion)
	case strings.HasPrefix(fromVersion, "1.19.") && strings.HasPrefix(toVersion, "1.19."):
		return hyperkubeUpgrade(ctx, cluster, arch, toVersion)
	default:
		return fmt.Errorf("unsupported upgrade from %q to %q", fromVersion, toVersion)
	}
}

// hyperkubeUpgrade upgrades from hyperkube-based to distroless images in 1.19.
func hyperkubeUpgrade(ctx context.Context, cluster cluster.K8sProvider, arch, targetVersion string) error {
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
		if err = hyperkubeUpgradeDs(ctx, clientset, ds, arch, targetVersion); err != nil {
			return fmt.Errorf("failed updating daemonset %q: %w", ds, err)
		}
	}

	if err = podCheckpointerGracePeriod(ctx, clientset, graceTimeout.String()); err != nil {
		return fmt.Errorf("error setting pod-checkpointer grace period: %w", err)
	}

	return nil
}

//nolint: gocyclo
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

func podCheckpointerGracePeriod(ctx context.Context, clientset *kubernetes.Clientset, gracePeriod string) error {
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

		return nil
	})
}

//nolint: gocyclo
func hyperkubeUpgradeDs(ctx context.Context, clientset *kubernetes.Clientset, ds, arch, targetVersion string) error {
	if ds == kubeAPIServer {
		fmt.Printf("temporarily taking %q out of pod-checkpointer control\n", ds)

		if err := updateDaemonset(ctx, clientset, ds, func(daemonset *appsv1.DaemonSet) error {
			delete(daemonset.Spec.Template.Annotations, checkpointerAnnotation)

			return nil
		}); err != nil {
			return err
		}
	}

	fmt.Printf("updating daemonset %q to version %q\n", ds, targetVersion)

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
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s-%s:v%s", constants.KubernetesAPIServerImage, arch, targetVersion)
		case kubeControllerManager:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s-%s:v%s", constants.KubernetesControllerManagerImage, arch, targetVersion)
		case kubeScheduler:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s-%s:v%s", constants.KubernetesSchedulerImage, arch, targetVersion)
		case kubeProxy:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s-%s:v%s", constants.KubernetesProxyImage, arch, targetVersion)
		default:
			return fmt.Errorf("failed to build new image spec")
		}

		if ds == kubeAPIServer {
			if daemonset.Spec.Template.Annotations == nil {
				daemonset.Spec.Template.Annotations = make(map[string]string)
			}

			daemonset.Spec.Template.Annotations[checkpointerAnnotation] = "true"
		}

		return nil
	})
}
