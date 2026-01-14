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

// PreBootSequenceChecks
const (
	CheckEtcdHealthy                  = "etcd to be healthy"
	CheckEtcdConsistent               = "etcd members to be consistent across nodes"
	CheckEtcdControlPlane             = "etcd members to be control plane nodes"
	CheckApidReady                    = "apid to be ready"
	CheckAllNodesMemorySizes          = "all nodes memory sizes"
	CheckAllNodesDiskSizes            = "all nodes disk sizes"
	CheckNoDiagnostics                = "no diagnostics"
	CheckKubeletHealthy               = "kubelet to be healthy"
	CheckAllNodesBootSequenceFinished = "all nodes to finish boot sequence"
)

// K8sComponentsReadinessChecks
const (
	CheckK8sAllNodesReported           = "all k8s nodes to report"
	CheckControlPlaneStaticPodsRunning = "all control plane static pods to be running"
	CheckControlPlaneComponentsReady   = "all control plane components to be ready"
)

// DefaultClusterChecks
const (
	CheckK8sAllNodesReady    = "all k8s nodes to report ready"
	CheckKubeProxyReady      = "kube-proxy to report ready"
	CheckCoreDNSReady        = "coredns to report ready"
	CheckK8sNodesSchedulable = "all k8s nodes to report schedulable"
)

func getCheck(name string) ClusterCheck {
	switch name {
	// PreBootSequenceChecks
	case CheckEtcdHealthy:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckEtcdHealthy, func(ctx context.Context) error {
				return ServiceHealthAssertion(ctx, cluster, "etcd", WithNodeTypes(machine.TypeInit, machine.TypeControlPlane))
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckEtcdConsistent:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckEtcdConsistent, func(ctx context.Context) error {
				return EtcdConsistentAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckEtcdControlPlane:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckEtcdControlPlane, func(ctx context.Context) error {
				return EtcdControlPlaneNodesAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckApidReady:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckApidReady, func(ctx context.Context) error {
				return ApidReadyAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckAllNodesMemorySizes:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckAllNodesMemorySizes, func(ctx context.Context) error {
				return AllNodesMemorySizes(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckAllNodesDiskSizes:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckAllNodesDiskSizes, func(ctx context.Context) error {
				return AllNodesDiskSizes(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckNoDiagnostics:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckNoDiagnostics, func(ctx context.Context) error {
				return NoDiagnostics(ctx, cluster)
			}, time.Minute, 5*time.Second)
		}
	case CheckKubeletHealthy:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckKubeletHealthy, func(ctx context.Context) error {
				return ServiceHealthAssertion(ctx, cluster, "kubelet", WithNodeTypes(machine.TypeInit, machine.TypeControlPlane))
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckAllNodesBootSequenceFinished:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckAllNodesBootSequenceFinished, func(ctx context.Context) error {
				return AllNodesBootedAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}

	// K8sComponentsReadinessChecks
	case CheckK8sAllNodesReported:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckK8sAllNodesReported, func(ctx context.Context) error {
				return K8sAllNodesReportedAssertion(ctx, cluster)
			}, 5*time.Minute, 30*time.Second)
		}
	case CheckControlPlaneStaticPodsRunning:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckControlPlaneStaticPodsRunning, func(ctx context.Context) error {
				return K8sControlPlaneStaticPods(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckControlPlaneComponentsReady:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckControlPlaneComponentsReady, func(ctx context.Context) error {
				return K8sFullControlPlaneAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}

	// Additional Checks for Default Cluster Checks
	case CheckK8sAllNodesReady:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckK8sAllNodesReady, func(ctx context.Context) error {
				return K8sAllNodesReadyAssertion(ctx, cluster)
			}, 10*time.Minute, 5*time.Second)
		}
	case CheckKubeProxyReady:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckKubeProxyReady, func(ctx context.Context) error {
				present, replicas, err := DaemonSetPresent(ctx, cluster, "kube-system", "k8s-app=kube-proxy")
				if err != nil {
					return err
				}
				if !present {
					return conditions.ErrSkipAssertion
				}
				return K8sPodReadyAssertion(ctx, cluster, replicas, "kube-system", "k8s-app=kube-proxy")
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckCoreDNSReady:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckCoreDNSReady, func(ctx context.Context) error {
				present, replicas, err := DeploymentPresent(ctx, cluster, "kube-system", "k8s-app=kube-dns")
				if err != nil {
					return err
				}
				if !present {
					return conditions.ErrSkipAssertion
				}
				return K8sPodReadyAssertion(ctx, cluster, replicas, "kube-system", "k8s-app=kube-dns")
			}, 5*time.Minute, 5*time.Second)
		}
	case CheckK8sNodesSchedulable:
		return func(cluster ClusterInfo) conditions.Condition {
			return conditions.PollingCondition(CheckK8sNodesSchedulable, func(ctx context.Context) error {
				return K8sAllNodesSchedulableAssertion(ctx, cluster)
			}, 5*time.Minute, 5*time.Second)
		}
	default:
		panic("unknown check name: " + name)
	}
}

// DefaultClusterChecks returns a set of default Talos cluster readiness checks.
func DefaultClusterChecks() []ClusterCheck {
	// Concatenate pre-boot, Kubernetes component, and additional checks.
	return slices.Concat(
		PreBootSequenceChecks(),
		K8sComponentsReadinessChecks(),
		[]ClusterCheck{
			// wait for all the nodes to report ready at k8s level
			getCheck(CheckK8sAllNodesReady),
			// wait for kube-proxy to report ready
			getCheck(CheckKubeProxyReady),
			// wait for coredns to report ready
			getCheck(CheckCoreDNSReady),
			// wait for all the nodes to be schedulable
			getCheck(CheckK8sNodesSchedulable),
		},
	)
}

// K8sComponentsReadinessChecks returns a set of K8s cluster readiness checks which are specific to the k8s components
// being up and running. This test can be skipped if the cluster is set to use a custom CNI, as the checks won't be healthy
// until the CNI is up and running.
func K8sComponentsReadinessChecks() []ClusterCheck {
	return []ClusterCheck{
		// wait for all the nodes to report in at k8s level
		getCheck(CheckK8sAllNodesReported),
		// wait for k8s control plane static pods
		getCheck(CheckControlPlaneStaticPodsRunning),
		// wait for HA k8s control plane
		getCheck(CheckControlPlaneComponentsReady),
	}
}

// ExtraClusterChecks returns a set of additional Talos cluster readiness checks which work only for newer versions of Talos.
//
// ExtraClusterChecks can't be used reliably in upgrade tests, as older versions might not pass the checks.
func ExtraClusterChecks() []ClusterCheck {
	return []ClusterCheck{}
}

// preBootSequenceCheckNames returns the list of pre-boot check names.
func preBootSequenceCheckNames() []string {
	return []string{
		CheckEtcdHealthy,
		CheckEtcdConsistent,
		CheckEtcdControlPlane,
		CheckApidReady,
		CheckAllNodesMemorySizes,
		CheckAllNodesDiskSizes,
		CheckNoDiagnostics,
		CheckKubeletHealthy,
		CheckAllNodesBootSequenceFinished,
	}
}

// PreBootSequenceChecks returns a set of Talos cluster readiness checks which are run before boot sequence.
func PreBootSequenceChecks() []ClusterCheck {
	return PreBootSequenceChecksFiltered(nil)
}

// PreBootSequenceChecksFiltered returns a filtered version of the PreBootSequenceChecks,
// removing any checks whose names appear in the provided 'skips' list.
func PreBootSequenceChecksFiltered(skips []string) []ClusterCheck {
	checkNames := []string{
		CheckEtcdHealthy,
		CheckEtcdConsistent,
		CheckEtcdControlPlane,
		CheckApidReady,
		CheckAllNodesMemorySizes,
		CheckAllNodesDiskSizes,
		CheckNoDiagnostics,
		CheckKubeletHealthy,
		CheckAllNodesBootSequenceFinished,
	}

	var filtered []ClusterCheck
	for _, name := range checkNames {
		if slices.Contains(skips, name) {
			continue
		}
		filtered = append(filtered, getCheck(name))
	}
	return filtered
}
