// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	_ "embed"

	"github.com/siderolabs/talos/pkg/flannel"
)

// kube-apiserver configuration:

//go:embed templates/kube-system-encryption-config-template.yaml
var kubeSystemEncryptionConfigTemplate string

// manifests injected into kube-apiserver

//go:embed templates/kubelet-bootstrapping-token-template.yaml
var kubeletBootstrappingToken string

// csrNodeBootstrapTemplate lets bootstrapping tokens and nodes request CSRs.
//
//go:embed templates/csr-node-bootstrap-template.yaml
var csrNodeBootstrapTemplate string

// csrApproverRoleBindingTemplate instructs the csrapprover controller to
// automatically approve CSRs made by bootstrapping tokens for client
// credentials.
//
// This binding should be removed to disable CSR auto-approval.
//
//go:embed templates/csr-approver-role-binding-template.yaml
var csrApproverRoleBindingTemplate string

// csrRenewalRoleBindingTemplate instructs the csrapprover controller to
// automatically approve all CSRs made by nodes to renew their client
// certificates.
//
// This binding should be altered in the future to hold a list of node
// names instead of targeting `system:nodes` so we can revoke individual
// node's ability to renew its certs.
//
//go:embed templates/csr-renewal-role-binding-template.yaml
var csrRenewalRoleBindingTemplate string

//go:embed templates/kube-proxy-template.yaml
var kubeProxyTemplate string

// kubeConfigInCluster instructs clients to use their service account token,
// but unlike an in-cluster client doesn't rely on the `KUBERNETES_SERVICE_PORT`
// and `KUBERNETES_PORT` to determine the API servers address.
//
// This kubeconfig is used by bootstrapping pods that might not have access to
// these env vars, such as kube-proxy, which sets up the API server endpoint
// (chicken and egg), and the checkpointer, which needs to run as a static pod
// even if the API server isn't available.
//
//go:embed templates/kube-config-in-cluster-template.yaml
var kubeConfigInClusterTemplate string

//go:embed templates/core-dns-template.yaml
var coreDNSTemplate string

//go:embed templates/core-dns-svc-template.yaml
var coreDNSSvcTemplate string

// podSecurityPolicy is the default PSP.
//
//go:embed templates/pod-security-policy-template.yaml
var podSecurityPolicy string

// talosAPIService is the service to access Talos API from Kubernetes.
// Service exposes the Endpoints which are managed by controllers.
//
//go:embed templates/talos-api-service-template.yaml
var talosAPIService string

var flannelTemplate = string(flannel.Template)

// talosServiceAccountCRDTemplate is the template of the CRD which
// allows injecting Talos with credentials into the Kubernetes cluster.
//
//go:embed templates/talos-service-account-crd-template.yaml
var talosServiceAccountCRDTemplate string
