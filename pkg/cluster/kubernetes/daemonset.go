// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/siderolabs/go-retry/retry"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"

	k8s "github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

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

			return fmt.Errorf("error fetching daemonset: %w", err)
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

func upgradeDaemonset(ctx context.Context, clientset *kubernetes.Clientset, ds string, options UpgradeOptions) error {
	options.Log("updating daemonset %q to version %q", ds, options.ToVersion)

	if options.DryRun {
		options.Log("skipped in dry-run")

		return nil
	}

	return updateDaemonset(ctx, clientset, ds, func(daemonset *appsv1.DaemonSet) error {
		if len(daemonset.Spec.Template.Spec.Containers) != 1 {
			return fmt.Errorf("unexpected number of containers: %d", len(daemonset.Spec.Template.Spec.Containers))
		}

		switch ds {
		case kubeProxy:
			daemonset.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("%s:v%s", constants.KubernetesProxyImage, options.ToVersion)
		default:
			return fmt.Errorf("failed to build new image spec")
		}

		for _, updater := range options.extraUpdaters {
			if err := updater(ds, daemonset); err != nil {
				return err
			}
		}

		return nil
	})
}
