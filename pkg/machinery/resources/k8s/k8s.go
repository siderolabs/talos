// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
package k8s

import "github.com/cosi-project/runtime/pkg/resource"

//go:generate deep-copy -type AdmissionControlConfigSpec -type APIServerConfigSpec -type AuditPolicyConfigSpec -type AuthorizationConfigSpec -type BootstrapManifestsConfigSpec -type ConfigStatusSpec -type ControllerManagerConfigSpec -type EndpointSpec -type ExtraManifestsConfigSpec -type KubeletLifecycleSpec -type KubePrismConfigSpec -type KubePrismEndpointsSpec -type KubePrismStatusesSpec -type KubeletSpecSpec -type ManifestSpec -type ManifestStatusSpec -type NodeAnnotationSpecSpec -type NodeCordonedSpecSpec -type NodeLabelSpecSpec -type NodeTaintSpecSpec -type KubeletConfigSpec -type NodeIPSpec -type NodeIPConfigSpec -type NodeStatusSpec -type NodenameSpec -type SchedulerConfigSpec -type SecretsStatusSpec -type StaticPodSpec -type StaticPodStatusSpec -type StaticPodServerStatusSpec  -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// NamespaceName contains resources supporting Kubernetes components on all node types.
const NamespaceName resource.Namespace = "k8s"

// ControlPlaneNamespaceName contains resources supporting Kubernetes control plane.
const ControlPlaneNamespaceName resource.Namespace = "controlplane"

// NodeAddressFilterOnlyK8s is the ID for the node address filter which leaves only Kubernetes IPs.
const NodeAddressFilterOnlyK8s = "only-k8s"

// NodeAddressFilterNoK8s is the ID for the node address filter which removes any Kubernetes IPs.
const NodeAddressFilterNoK8s = "no-k8s"

// APIServerID is a generic ID for resources related to kube-apiserver.
const APIServerID = "kube-apiserver"

// ControllerManagerID is a generic ID for resources related to kube-controller-manager.
const ControllerManagerID = "kube-controller-manager"

// SchedulerID is a generic ID for resources related to kube-scheduler.
const SchedulerID = "kube-scheduler"
