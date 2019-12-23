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

	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
)

// K8sAllNodesReportedAssertion checks whether all the nodes show up in node list.
func K8sAllNodesReportedAssertion(ctx context.Context, cluster provision.ClusterAccess) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	expectedNodes := make([]string, 0, len(cluster.Info().Nodes))

	for _, node := range cluster.Info().Nodes {
		expectedNodes = append(expectedNodes, node.PrivateIP.String())
	}

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
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
func K8sFullControlPlaneAssertion(ctx context.Context, cluster provision.ClusterAccess) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	var expectedNodes []string

	for _, node := range cluster.Info().Nodes {
		if node.Type == generate.TypeInit || node.Type == generate.TypeControlPlane {
			expectedNodes = append(expectedNodes, node.PrivateIP.String())
		}
	}

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	var actualNodes []string

	for _, node := range nodes.Items {
		for label := range node.Labels {
			if label == "node-role.kubernetes.io/master" {
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

	if reflect.DeepEqual(expectedNodes, actualNodes) {
		return nil
	}

	return fmt.Errorf("expected %v nodes, but got %v nodes", expectedNodes, actualNodes)
}

// K8sAllNodesReadyAssertion checks whether all the nodes are Ready.
func K8sAllNodesReadyAssertion(ctx context.Context, cluster provision.ClusterAccess) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
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

// K8sPodReadyAssertion checks whether all the nodes are Ready.
func K8sPodReadyAssertion(ctx context.Context, cluster provision.ClusterAccess, namespace, labelSelector string) error {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
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
