// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"slices"
	"time"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// DefaultClusterChecks returns a set of default Talos cluster readiness checks.
func DefaultClusterChecks() []ClusterCheck {
	return slices.Concat(
		PreBootSequenceChecks(),
		K8sComponentsReadinessChecks(),
		[]ClusterCheck{
			// wait for all the nodes to report ready at k8s level
			func(cluster ClusterInfo) conditions.Condition {
				return conditions.PollingCondition("all k8s nodes to report ready", func(ctx context.Context) error {
					return K8sAllNodesReadyAssertion(ctx, cluster)
				}, 10*time.Minute, 5*time.Second)
			},

			// wait for kube-proxy to report ready
			func(cluster ClusterInfo) conditions.Condition {
				return conditions.PollingCondition("kube-proxy to report ready", func(ctx context.Context) error {
					present, replicas, err := DaemonSetPresent(ctx, cluster, "kube-system", "k8s-app=kube-proxy")
					if err != nil {
						return err
					}

					if !present {
						return conditions.ErrSkipAssertion
					}

					return K8sPodReadyAssertion(ctx, cluster, replicas, "kube-system", "k8s-app=kube-proxy")
				}, 5*time.Minute, 5*time.Second)
			},

			// wait for coredns to report ready
			func(cluster ClusterInfo) conditions.Condition {
				return conditions.PollingCondition("coredns to report ready", func(ctx context.Context) error {
					present, replicas, err := ReplicaSetPresent(ctx, cluster, "kube-system", "k8s-app=kube-dns")
					if err != nil {
						return err
					}

					if !present {
						return conditions.ErrSkipAssertion
					}

					return K8sPodReadyAssertion(ctx, cluster, replicas, "kube-system", "k8s-app=kube-dns")
				}, 5*time.Minute, 5*time.Second)
			},

			// wait for all the nodes to be schedulable
			func(cluster ClusterInfo) conditions.Condition {
				return conditions.PollingCondition("all k8s nodes to report schedulable", func(ctx context.Context) error {
					return K8sAllNodesSchedulableAssertion(ctx, cluster)
				}, 5*time.Minute, 5*time.Second)
			},
		},
	)
}

// K8sComponentsReadinessChecks returns a set of K8s cluster readiness checks which are specific to the k8s components
// being up and running. This test can be skipped if the cluster is set to use a custom CNI, as the checks won't be healthy
// until the CNI is up and running.
func K8sComponentsReadinessChecks() []ClusterCheck {
	return []ClusterCheck{
		// wait for all the nodes to report in at k8s level
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("all k8s nodes to report", func(ctx context.Context) error {
				return K8sAllNodesReportedAssertion(ctx, cluster)
			}, 5*time.Minute, 30*time.Second) // give more time per each attempt, as this check is going to build and cache kubeconfig
		},

		// wait for k8s control plane static pods
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("all control plane static pods to be running", func(ctx context.Context) error {
				return K8sControlPlaneStaticPods(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},

		// wait for HA k8s control plane
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("all control plane components to be ready", func(ctx context.Context) error {
				return K8sFullControlPlaneAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},
	}
}

// ExtraClusterChecks returns a set of additional Talos cluster readiness checks which work only for newer versions of Talos.
//
// ExtraClusterChecks can't be used reliably in upgrade tests, as older versions might not pass the checks.
func ExtraClusterChecks() []ClusterCheck {
	return []ClusterCheck{}
}

// PreBootSequenceChecks returns a set of Talos cluster readiness checks which are run before boot sequence.
func PreBootSequenceChecks() []ClusterCheck {
	return []ClusterCheck{
		// wait for etcd to be healthy on all control plane nodes
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("etcd to be healthy", func(ctx context.Context) error {
				return ServiceHealthAssertion(ctx, cluster, "etcd", WithNodeTypes(machine.TypeInit, machine.TypeControlPlane))
			}, 5*time.Minute, 5*time.Second)
		},

		// wait for etcd members to be consistent across nodes
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("etcd members to be consistent across nodes", func(ctx context.Context) error {
				return EtcdConsistentAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},

		// wait for etcd members to be the control plane nodes
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("etcd members to be control plane nodes", func(ctx context.Context) error {
				return EtcdControlPlaneNodesAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},

		// wait for apid to be ready on all the nodes
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("apid to be ready", func(ctx context.Context) error {
				return ApidReadyAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},

		// wait for all nodes to report their memory size
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("all nodes memory sizes", func(ctx context.Context) error {
				return AllNodesMemorySizes(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},

		// wait for all nodes to report their disk size
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("all nodes disk sizes", func(ctx context.Context) error {
				return AllNodesDiskSizes(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},

		// check diagnostics
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("no diagnostics", func(ctx context.Context) error {
				return NoDiagnostics(ctx, cluster)
			}, time.Minute, 5*time.Second)
		},

		// wait for kubelet to be healthy on all
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("kubelet to be healthy", func(ctx context.Context) error {
				return ServiceHealthAssertion(ctx, cluster, "kubelet", WithNodeTypes(machine.TypeInit, machine.TypeControlPlane))
			}, 5*time.Minute, 5*time.Second)
		},

		// wait for all nodes to finish booting
		func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition("all nodes to finish boot sequence", func(ctx context.Context) error {
				return AllNodesBootedAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		},
	}
}
