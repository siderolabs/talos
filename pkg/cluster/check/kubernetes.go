// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package check provides set of checks to verify cluster readiness.
package check

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"google.golang.org/grpc/codes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// K8sAllNodesReportedAssertion checks whether all the nodes show up in node list.
func K8sAllNodesReportedAssertion(ctx context.Context, cl ClusterInfo) error {
	clientset, err := cl.K8sClient(ctx)
	if err != nil {
		return err
	}

	expectedNodeInfos := cl.Nodes()

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	actualNodeInfos := make([]cluster.NodeInfo, 0, len(nodes.Items))

	for _, node := range nodes.Items {
		var internalIP netip.Addr

		var ips []netip.Addr

		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == v1.NodeInternalIP {
				internalIP, err = netip.ParseAddr(nodeAddress.Address)
				if err != nil {
					return err
				}

				ips = append(ips, internalIP)
			} else if nodeAddress.Type == v1.NodeExternalIP {
				externalIP, err := netip.ParseAddr(nodeAddress.Address)
				if err != nil {
					return err
				}

				ips = append(ips, externalIP)
			}
		}

		actualNodeInfo := cluster.NodeInfo{
			InternalIP: internalIP,
			IPs:        ips,
		}

		actualNodeInfos = append(actualNodeInfos, actualNodeInfo)
	}

	return cluster.NodesMatch(expectedNodeInfos, actualNodeInfos)
}

// K8sFullControlPlaneAssertion checks whether all the controlplane nodes are k8s controlplane nodes.
//
//nolint:gocyclo,cyclop
func K8sFullControlPlaneAssertion(ctx context.Context, cl ClusterInfo) error {
	clientset, err := cl.K8sClient(ctx)
	if err != nil {
		return err
	}

	expectedNodes := append(cl.NodesByType(machine.TypeInit), cl.NodesByType(machine.TypeControlPlane)...)

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	actualNodes := make([]cluster.NodeInfo, 0, len(nodes.Items))

	for _, node := range nodes.Items {
		for label := range node.Labels {
			if label == constants.LabelNodeRoleControlPlane {
				var internalIP netip.Addr

				var ips []netip.Addr

				for _, nodeAddress := range node.Status.Addresses {
					if nodeAddress.Type == v1.NodeInternalIP {
						internalIP, err = netip.ParseAddr(nodeAddress.Address)
						if err != nil {
							return err
						}

						ips = append(ips, internalIP)
					} else if nodeAddress.Type == v1.NodeExternalIP {
						externalIP, err2 := netip.ParseAddr(nodeAddress.Address)
						if err2 != nil {
							return err2
						}

						ips = append(ips, externalIP)
					}
				}

				actualNodeInfo := cluster.NodeInfo{
					InternalIP: internalIP,
					IPs:        ips,
				}

				actualNodes = append(actualNodes, actualNodeInfo)

				break
			}
		}
	}

	err = cluster.NodesMatch(expectedNodes, actualNodes)
	if err != nil {
		return err
	}

	// NB: We run the control plane check after node readiness check in order to
	// ensure that all control plane nodes have been labeled with the controlplane
	// label.

	// daemonset check only there for pre-0.9 clusters with self-hosted control plane
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
	}

	for _, k8sApp := range []string{"kube-apiserver", "kube-scheduler", "kube-controller-manager"} {
		// list pods to verify that daemonset status is updated properly
		pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("k8s-app = %s", k8sApp),
		})
		if err != nil {
			return fmt.Errorf("error listing pods for app %s: %w", k8sApp, err)
		}

		// filter out pod checkpoints
		n := 0

		for _, pod := range pods.Items {
			if _, exists := pod.Annotations["checkpointer.alpha.coreos.com/checkpoint-of"]; !exists {
				pods.Items[n] = pod
				n++
			}
		}

		pods.Items = pods.Items[:n]

		if len(pods.Items) != len(actualNodes) {
			return fmt.Errorf("expected number of pods for %s to be %d, got %d",
				k8sApp, len(actualNodes), len(pods.Items))
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
			return fmt.Errorf("some pods are not ready for %s: %v", k8sApp, notReadyPods)
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

// K8sPodReadyAssertion checks whether all the pods matching label selector are Ready, and there is at least one.
//
//nolint:gocyclo
func K8sPodReadyAssertion(ctx context.Context, cluster cluster.K8sProvider, replicas int, namespace, labelSelector string) error {
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
		return fmt.Errorf("no pods found for namespace %q and label selector %q", namespace, labelSelector)
	}

	var notReadyPods, readyPods []string

	for _, pod := range pods.Items {
		// skip deleted pods
		if pod.DeletionTimestamp != nil {
			continue
		}

		// skip failed pods
		if pod.Status.Phase == v1.PodFailed {
			continue
		}

		// skip pods which `kubectl get pods` marks as 'Completed':
		// * these pods have a phase 'Running', but all containers are terminated
		// * such pods appear after a graceful kubelet shutdown
		allContainersTerminated := true

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Terminated == nil {
				allContainersTerminated = false

				break
			}
		}

		if allContainersTerminated {
			continue
		}

		ready := false

		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady {
				if cond.Status == v1.ConditionTrue {
					ready = true

					break
				}
			}
		}

		if !ready {
			notReadyPods = append(notReadyPods, pod.Name)
		} else {
			readyPods = append(readyPods, pod.Name)
		}
	}

	if len(readyPods) == 0 {
		return fmt.Errorf("no ready pods found for namespace %q and label selector %q", namespace, labelSelector)
	}

	if len(notReadyPods) == 0 {
		if len(readyPods) != replicas {
			return fmt.Errorf("expected %d ready pods, got %d", replicas, len(readyPods))
		}

		return nil
	}

	return fmt.Errorf("some pods are not ready: %v", notReadyPods)
}

// DaemonSetPresent returns true if there is at least one DaemonSet matching given label selector.
//
//nolint:dupl
func DaemonSetPresent(ctx context.Context, cluster cluster.K8sProvider, namespace, labelSelector string) (bool, int, error) {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return false, 0, err
	}

	dss, err := clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, 0, err
	}

	if len(dss.Items) == 0 {
		return false, 0, nil
	}

	return true, int(dss.Items[0].Status.DesiredNumberScheduled), nil
}

// ReplicaSetPresent returns true if there is at least one ReplicaSet matching given label selector.
//
//nolint:dupl
func ReplicaSetPresent(ctx context.Context, cluster cluster.K8sProvider, namespace, labelSelector string) (bool, int, error) {
	clientset, err := cluster.K8sClient(ctx)
	if err != nil {
		return false, 0, err
	}

	rss, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return false, 0, err
	}

	if len(rss.Items) == 0 {
		return false, 0, nil
	}

	return true, int(rss.Items[0].Status.Replicas), nil
}

// K8sControlPlaneStaticPods checks whether all the controlplane nodes are running required Kubernetes static pods.
func K8sControlPlaneStaticPods(ctx context.Context, cl ClusterInfo) error {
	expectedNodes := append(cl.NodesByType(machine.TypeInit), cl.NodesByType(machine.TypeControlPlane)...)

	// using here new Talos COSI API, Talos 1.2+ required
	c, err := cl.Client()
	if err != nil {
		return err
	}

	for _, node := range expectedNodes {
		expectedStaticPods := map[string]struct{}{
			"kube-system/kube-apiserver":          {},
			"kube-system/kube-controller-manager": {},
			"kube-system/kube-scheduler":          {},
		}

		items, err := safe.StateListAll[*k8s.StaticPodStatus](client.WithNode(ctx, node.InternalIP.String()), c.COSI)
		if err != nil {
			if client.StatusCode(err) == codes.Unimplemented {
				// old version of Talos without COSI API
				return nil
			}

			return fmt.Errorf("error listing static pods on node %s: %w", node.InternalIP, err)
		}

		for iter := items.Iterator(); iter.Next(); {
			for expectedStaticPod := range expectedStaticPods {
				if strings.HasPrefix(iter.Value().Metadata().ID(), expectedStaticPod) {
					delete(expectedStaticPods, expectedStaticPod)
				}
			}
		}

		if len(expectedStaticPods) > 0 {
			missingStaticPods := maps.Keys(expectedStaticPods)
			slices.Sort(missingStaticPods)

			return fmt.Errorf("missing static pods on node %s: %v", node.InternalIP, missingStaticPods)
		}
	}

	return nil
}
