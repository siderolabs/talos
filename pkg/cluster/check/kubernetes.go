// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/talos-systems/talos/pkg/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// K8sAllNodesReportedAssertion checks whether all the nodes show up in node list.
func K8sAllNodesReportedAssertion(ctx context.Context, cluster ClusterInfo) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	expectedNodes := cluster.Nodes()

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var actualNodes []string

	for _, node := range nodes.Items {
		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == v1.NodeInternalIP {
				actualNodes = append(actualNodes, nodeAddress.Address)
			}
		}
	}

	sort.Strings(expectedNodes)
	sort.Strings(actualNodes)

	if reflect.DeepEqual(expectedNodes, actualNodes) {
		return nil
	}

	return fmt.Errorf("expected %v nodes, but got %v nodes", expectedNodes, actualNodes)
}

// K8sFullControlPlaneAssertion checks whether all the master nodes are k8s master nodes.
//
//nolint: gocyclo
func K8sFullControlPlaneAssertion(ctx context.Context, cluster ClusterInfo) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	expectedNodes := append(cluster.NodesByType(machine.TypeInit), cluster.NodesByType(machine.TypeControlPlane)...)

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var actualNodes []string

	for _, node := range nodes.Items {
		for label := range node.Labels {
			if label == constants.LabelNodeRoleMaster {
				for _, nodeAddress := range node.Status.Addresses {
					if nodeAddress.Type == v1.NodeInternalIP {
						actualNodes = append(actualNodes, nodeAddress.Address)
						break
					}
				}

				break
			}
		}
	}

	sort.Strings(expectedNodes)
	sort.Strings(actualNodes)

	if !reflect.DeepEqual(expectedNodes, actualNodes) {
		return fmt.Errorf("expected %v nodes, but got %v nodes", expectedNodes, actualNodes)
	}

	// NB: We run the control plane check after node readiness check in order to
	// ensure that all control plane nodes have been labeled with the master
	// label.

	daemonsets, err := clientset.AppsV1().DaemonSets("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app in (kube-apiserver,kube-scheduler,kube-controller-manager)",
	})
	if err != nil {
		return err
	}

	for _, ds := range daemonsets.Items {
		if ds.Status.CurrentNumberScheduled != ds.Status.DesiredNumberScheduled {
			return fmt.Errorf("expected current number scheduled for %s to be %d, got %d", ds.GetName(), ds.Status.DesiredNumberScheduled, ds.Status.CurrentNumberScheduled)
		}

		if ds.Status.NumberAvailable != ds.Status.DesiredNumberScheduled {
			return fmt.Errorf("expected number available for %s to be %d, got %d", ds.GetName(), ds.Status.DesiredNumberScheduled, ds.Status.NumberAvailable)
		}

		if ds.Status.NumberReady != ds.Status.DesiredNumberScheduled {
			return fmt.Errorf("expected number ready for %s to be %d, got %d", ds.GetName(), ds.Status.DesiredNumberScheduled, ds.Status.NumberReady)
		}

		// list pods to verify that daemonset status is updated properly
		pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("k8s-app = %s", ds.Labels["k8s-app"]),
		})
		if err != nil {
			return fmt.Errorf("error listing pods for daemonset %s: %w", ds.GetName(), err)
		}

		if int32(len(pods.Items)) != ds.Status.DesiredNumberScheduled {
			return fmt.Errorf("expected number of pods for %s to be %d, got %d", ds.GetName(), ds.Status.DesiredNumberScheduled, len(pods.Items))
		}

		var notReadyPods []string

		for _, pod := range pods.Items {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == v1.PodReady {
					if cond.Status != v1.ConditionTrue {
						notReadyPods = append(notReadyPods, pod.Name)
						break
					}
				}
			}
		}

		if len(notReadyPods) > 0 {
			return fmt.Errorf("some pods are not ready for %s: %v", ds.GetName(), notReadyPods)
		}
	}

	return nil
}

// K8sAllNodesReadyAssertion checks whether all the nodes are Ready.
func K8sAllNodesReadyAssertion(ctx context.Context, cluster cluster.K8sProvider) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var notReadyNodes []string

	for _, node := range nodes.Items {
		for _, cond := range node.Status.Conditions {
			if cond.Type == v1.NodeReady {
				if cond.Status != v1.ConditionTrue {
					notReadyNodes = append(notReadyNodes, node.Name)
					break
				}
			}
		}
	}

	if len(notReadyNodes) == 0 {
		return nil
	}

	return fmt.Errorf("some nodes are not ready: %v", notReadyNodes)
}

// K8sAllNodesSchedulableAssertion checks whether all the nodes are schedulable (not cordoned).
func K8sAllNodesSchedulableAssertion(ctx context.Context, cluster cluster.K8sProvider) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	var notSchedulableNodes []string

	for _, node := range nodes.Items {
		if node.Spec.Unschedulable {
			notSchedulableNodes = append(notSchedulableNodes, node.Name)
			break
		}
	}

	if len(notSchedulableNodes) == 0 {
		return nil
	}

	return fmt.Errorf("some nodes are not schedulable: %v", notSchedulableNodes)
}

// K8sPodReadyAssertion checks whether all the pods are Ready.
func K8sPodReadyAssertion(ctx context.Context, cluster cluster.K8sProvider, namespace, labelSelector string) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for namespace %q and label %q", namespace, labelSelector)
	}

	var notReadyPods []string

	for _, pod := range pods.Items {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady {
				if cond.Status != v1.ConditionTrue {
					notReadyPods = append(notReadyPods, pod.Name)
					break
				}
			}
		}
	}

	if len(notReadyPods) == 0 {
		return nil
	}

	return fmt.Errorf("some pods are not ready: %v", notReadyPods)
}
