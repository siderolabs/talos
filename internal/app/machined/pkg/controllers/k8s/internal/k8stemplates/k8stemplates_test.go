// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8stemplates"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestTemplates(t *testing.T) {
	t.Parallel()

	const recordResults = false

	if recordResults {
		t.Log("recording test is enabled, failing the test")
		t.Fail()
	}

	for _, test := range []struct {
		name string
		obj  func() runtime.Object
	}{
		{
			name: "apiserver-encryption-secretbox",
			obj: func() runtime.Object {
				return k8stemplates.APIServerEncryptionConfig(&secrets.KubernetesRootSpec{
					SecretboxEncryptionSecret: "/FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
				})
			},
		},
		{
			name: "apiserver-encryption-aescbc",
			obj: func() runtime.Object {
				return k8stemplates.APIServerEncryptionConfig(&secrets.KubernetesRootSpec{
					AESCBCEncryptionSecret: "/sFYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
				})
			},
		},
		{
			name: "coredns-service-ipv4",
			obj: func() runtime.Object {
				return k8stemplates.CoreDNSService(&k8s.BootstrapManifestsConfigSpec{
					DNSServiceIP: "10.96.0.10",
				})
			},
		},
		{
			name: "coredns-service-ipv6",
			obj: func() runtime.Object {
				return k8stemplates.CoreDNSService(&k8s.BootstrapManifestsConfigSpec{
					DNSServiceIPv6: "fd00::10",
				})
			},
		},
		{
			name: "coredns-service-dual",
			obj: func() runtime.Object {
				return k8stemplates.CoreDNSService(&k8s.BootstrapManifestsConfigSpec{
					DNSServiceIP:   "10.96.0.10",
					DNSServiceIPv6: "fd00::10",
				})
			},
		},
		{
			name: "coredns-service-account",
			obj:  k8stemplates.CoreDNSServiceAccount,
		},
		{
			name: "coredns-cluster-role-binding",
			obj:  k8stemplates.CoreDNSClusterRoleBinding,
		},
		{
			name: "coredns-cluster-role",
			obj:  k8stemplates.CoreDNSClusterRole,
		},
		{
			name: "coredns-configmap-cluster-domain",
			obj: func() runtime.Object {
				return k8stemplates.CoreDNSConfigMap(&k8s.BootstrapManifestsConfigSpec{
					ClusterDomain: "cluster.local",
				})
			},
		},
		{
			name: "coredns-configmap-no-cluster-domain",
			obj: func() runtime.Object {
				return k8stemplates.CoreDNSConfigMap(&k8s.BootstrapManifestsConfigSpec{})
			},
		},
		{
			name: "coredns-deployment",
			obj: func() runtime.Object {
				return k8stemplates.CoreDNSDeployment(&k8s.BootstrapManifestsConfigSpec{
					CoreDNSImage: "coredns/coredns:1.9.3",
				})
			},
		},
		{
			name: "kubelet-bootstrapping-token",
			obj: func() runtime.Object {
				return k8stemplates.KubeletBootstrapTokenSecret(&secrets.KubernetesRootSpec{
					BootstrapTokenID:     "25p8ak",
					BootstrapTokenSecret: "vshybadgp2mhtvm7",
				})
			},
		},
		{
			name: "csr-node-bootstrap",
			obj:  k8stemplates.CSRNodeBootstrapTemplate,
		},
		{
			name: "csr-approver-role-binding",
			obj:  k8stemplates.CSRApproverRoleBindingTemplate,
		},
		{
			name: "csr-renewal-role-binding",
			obj:  k8stemplates.CSRRenewalRoleBindingTemplate,
		},
		{
			name: "kubeconfig-in-cluster",
			obj: func() runtime.Object {
				return k8stemplates.KubeconfigInClusterTemplate(&k8s.BootstrapManifestsConfigSpec{
					Server: "https://localhost:6443",
				})
			},
		},
		{
			name: "talos-nodes-rbac-cluster-role-binding",
			obj:  k8stemplates.TalosNodesRBACClusterRoleBinding,
		},
		{
			name: "talos-nodes-rbac-cluster-role",
			obj:  k8stemplates.TalosNodesRBACClusterRole,
		},
		{
			name: "kube-proxy-daemonset",
			obj: func() runtime.Object {
				return k8stemplates.KubeProxyDaemonSetTemplate(&k8s.BootstrapManifestsConfigSpec{
					ProxyImage: "k8s.gcr.io/kube-proxy:v1.27.0",
					ProxyArgs:  []string{"--proxy-mode=iptables"},
				})
			},
		},
		{
			name: "kube-proxy-service-account",
			obj:  k8stemplates.KubeProxyServiceAccount,
		},
		{
			name: "kube-proxy-cluster-role-binding",
			obj:  k8stemplates.KubeProxyClusterRoleBinding,
		},
		{
			name: "talos-service-account-crd",
			obj:  k8stemplates.TalosServiceAccountCRDTemplate,
		},
		{
			name: "flannel-cluster-role",
			obj:  k8stemplates.FlannelClusterRoleTemplate,
		},
		{
			name: "flannel-cluster-role-binding",
			obj:  k8stemplates.FlannelClusterRoleBindingTemplate,
		},
		{
			name: "flannel-service-account",
			obj:  k8stemplates.FlannelServiceAccountTemplate,
		},
		{
			name: "flannel-configmap-v4",
			obj: func() runtime.Object {
				return k8stemplates.FlannelConfigMapTemplate(&k8s.BootstrapManifestsConfigSpec{
					PodCIDRs: []string{"10.96.0.0/12"},
				})
			},
		},
		{
			name: "flannel-configmap-v6",
			obj: func() runtime.Object {
				return k8stemplates.FlannelConfigMapTemplate(&k8s.BootstrapManifestsConfigSpec{
					PodCIDRs: []string{"fd00::/112"},
				})
			},
		},
		{
			name: "flannel-configmap-dual",
			obj: func() runtime.Object {
				return k8stemplates.FlannelConfigMapTemplate(&k8s.BootstrapManifestsConfigSpec{
					PodCIDRs: []string{"10.96.0.0/12", "fd00::/112"},
				})
			},
		},
		{
			name: "flannel-daemonset",
			obj: func() runtime.Object {
				return k8stemplates.FlannelDaemonSetTemplate(&k8s.BootstrapManifestsConfigSpec{
					FlannelImage:     "quay.io/coreos/flannel:v0.14.0",
					FlannelExtraArgs: []string{"--foo=bar"},
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			obj := test.obj()

			out, err := k8stemplates.Marshal(obj)
			require.NoError(t, err)

			goldenPath := filepath.Join("testdata", test.name+".yaml")

			if recordResults {
				require.NoError(t, os.WriteFile(goldenPath, out, 0o644), "failed to write golden file %s", goldenPath)
			} else {
				golden, err := os.ReadFile(goldenPath)
				require.NoError(t, err, "failed to read golden file %s", goldenPath)

				require.Equal(t, string(golden), string(out), "output does not match golden file %s", goldenPath)
			}
		})
	}
}
