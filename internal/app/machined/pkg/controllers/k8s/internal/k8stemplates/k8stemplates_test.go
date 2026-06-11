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
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
				obj, err := k8stemplates.APIServerEncryptionConfig(&secrets.KubernetesRootSpec{
					SecretboxEncryptionSecret: "/FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
				})
				require.NoError(t, err)

				return obj
			},
		},
		{
			name: "apiserver-encryption-aescbc",
			obj: func() runtime.Object {
				obj, err := k8stemplates.APIServerEncryptionConfig(&secrets.KubernetesRootSpec{
					AESCBCEncryptionSecret: "/sFYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
				})
				require.NoError(t, err)

				return obj
			},
		},
		{
			name: "apiserver-encryption-config",
			obj: func() runtime.Object {
				obj, err := k8stemplates.APIServerEncryptionConfig(&secrets.KubernetesRootSpec{
					EtcdEncryptionConfig: map[string]any{
						"resources": []any{
							map[string]any{
								"resources": []string{"secrets"},
								"providers": []any{
									map[string]any{
										"secretbox": map[string]any{
											"keys": []any{
												map[string]any{
													"name":   "key2",
													"secret": "/FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
												},
											},
										},
									},
									map[string]any{
										"aescbc": map[string]any{
											"keys": []any{
												map[string]any{
													"name":   "key1",
													"secret": "/sFYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
												},
											},
										},
									},
								},
							},
						},
					},
				})
				require.NoError(t, err)

				return obj
			},
		},
		{
			name: "controller-manager",
			obj: func() runtime.Object {
				cfg := k8s.NewControllerManagerConfig(k8s.FinalControllerManagerConfigID)
				*cfg.TypedSpec() = k8s.ControllerManagerConfigSpec{
					Enabled:       true,
					Image:         "registry.k8s.io/controller-manager:v1.36.0",
					CloudProvider: "external",
					PodCIDRs:      []string{"10.96.0.0/12"},
					ServiceCIDRs:  []string{"10.224.0.0/16"},
					Args: []string{
						"/usr/local/bin/kube-controller-manager",
						"--use-service-account-credentials",
						"--allocate-node-cidrs=true",
					},
					EnvironmentVariables: map[string]string{
						"HTTP_PROXY": "http://127.0.0.1:443",
					},
					Resources: k8s.Resources{
						Requests: map[string]string{
							"cpu":    "50m",
							"memory": "500Mi",
						},
						Limits: map[string]string{
							"cpu":    "1",
							"memory": "1000Mi",
						},
					},
				}

				obj, err := k8stemplates.ControllerManagerPod(cfg, "111")
				require.NoError(t, err)

				return obj
			},
		},
		{
			name: "scheduler",
			obj: func() runtime.Object {
				cfg := k8s.NewSchedulerConfig(k8s.FinalSchedulerConfigID)
				*cfg.TypedSpec() = k8s.SchedulerConfigSpec{
					Enabled: true,
					Image:   "registry.k8s.io/scheduler:v1.36.0",
					Args: []string{
						"/usr/local/bin/kube-scheduler",
						"--authentication-kubeconfig=",
						"--authentication-tolerate-lookup-failure=false",
						"--authorization-kubeconfig=",
						"--bind-address=127.0.0.1",
						"--config=",
						"--leader-elect=true",
						"--profiling=false",
						"--tls-min-version=VersionTLS13",
					},
					EnvironmentVariables: map[string]string{
						"HTTP_PROXY": "http://127.0.0.1:443",
					},
					Resources: k8s.Resources{
						Requests: map[string]string{
							"cpu":    "50m",
							"memory": "500Mi",
						},
					},
				}

				obj, err := k8stemplates.SchedulerPod(cfg, "222")
				require.NoError(t, err)

				return obj
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
				spec, err := k8stemplates.KubeProxyDaemonSetTemplate(&k8s.BootstrapManifestsConfigSpec{
					ProxyImage: "k8s.gcr.io/kube-proxy:v1.27.0",
					ProxyArgs:  []string{"--proxy-mode=iptables"},
				})
				require.NoError(t, err)

				return spec
			},
		},
		{
			name: "kube-proxy-daemonset-with-config",
			obj: func() runtime.Object {
				spec, err := k8stemplates.KubeProxyDaemonSetTemplate(&k8s.BootstrapManifestsConfigSpec{
					ProxyImage: "k8s.gcr.io/kube-proxy:v1.27.0",
					ProxyArgs:  []string{"--config=/var/lib/kube-proxy/config.conf", "--hostname-override=$(NODE_NAME)"},
					ProxyConfig: map[string]any{
						"apiVersion": "kubeproxy.config.k8s.io/v1alpha1",
						"kind":       "KubeProxyConfiguration",
						"mode":       "nftables",
					},
					ProxyResources: k8s.Resources{
						Requests: map[string]string{
							"cpu":    "150m",
							"memory": "64Mi",
						},
						Limits: map[string]string{
							"cpu":    "300m",
							"memory": "128Mi",
						},
					},
					ProxyConfigChecksum: "abc123",
				})
				require.NoError(t, err)

				return spec
			},
		},
		{
			name: "kube-proxy-configmap",
			obj: func() runtime.Object {
				spec, err := k8stemplates.KubeProxyConfigMapTemplate(&k8s.BootstrapManifestsConfigSpec{
					ProxyConfig: map[string]any{
						"apiVersion":  "kubeproxy.config.k8s.io/v1alpha1",
						"kind":        "KubeProxyConfiguration",
						"mode":        "nftables",
						"clusterCIDR": "10.244.0.0/16",
						"clientConnection": map[string]any{
							"kubeconfig": "/etc/kubernetes/kubeconfig",
						},
						"conntrack": map[string]any{
							"maxPerCore": int32(0),
						},
					},
					ProxyConfigChecksum: "abc123",
				})
				require.NoError(t, err)

				return spec
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
			obj: func() runtime.Object {
				return k8stemplates.FlannelClusterRoleTemplate(&k8s.BootstrapManifestsConfigSpec{})
			},
		},
		{
			name: "flannel-cluster-role-with-network-policies",
			obj: func() runtime.Object {
				return k8stemplates.FlannelClusterRoleTemplate(&k8s.BootstrapManifestsConfigSpec{
					FlannelKubeNetworkPoliciesEnabled: true,
				})
			},
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
					PodCIDRs:           []string{"10.96.0.0/12"},
					FlannelBackendType: constants.FlannelDefaultBackend,
					FlannelBackendPort: constants.FlannelDefaultBackendPort,
				})
			},
		},
		{
			name: "flannel-configmap-v6",
			obj: func() runtime.Object {
				return k8stemplates.FlannelConfigMapTemplate(&k8s.BootstrapManifestsConfigSpec{
					PodCIDRs:           []string{"fd00::/112"},
					FlannelBackendType: constants.FlannelDefaultBackend,
					FlannelBackendPort: constants.FlannelDefaultBackendPort,
				})
			},
		},
		{
			name: "flannel-configmap-dual",
			obj: func() runtime.Object {
				return k8stemplates.FlannelConfigMapTemplate(&k8s.BootstrapManifestsConfigSpec{
					PodCIDRs:           []string{"10.96.0.0/12", "fd00::/112"},
					FlannelBackendType: constants.FlannelDefaultBackend,
					FlannelBackendPort: constants.FlannelDefaultBackendPort,
					FlannelBackendExtraConfig: map[string]any{
						"VNI": 4096,
					},
				})
			},
		},
		{
			name: "flannel-configmap-with-mtu",
			obj: func() runtime.Object {
				return k8stemplates.FlannelConfigMapTemplate(&k8s.BootstrapManifestsConfigSpec{
					PodCIDRs:           []string{"10.96.0.0/12"},
					FlannelBackendType: constants.FlannelDefaultBackend,
					FlannelBackendPort: constants.FlannelDefaultBackendPort,
					FlannelBackendMTU:  1420,
				})
			},
		},
		{
			name: "flannel-daemonset",
			obj: func() runtime.Object {
				spec, err := k8stemplates.FlannelDaemonSetTemplate(&k8s.BootstrapManifestsConfigSpec{
					FlannelImage:     "quay.io/coreos/flannel:v0.14.0",
					FlannelExtraArgs: []string{"--foo=bar"},
				})
				require.NoError(t, err)

				return spec
			},
		},
		{
			name: "flannel-daemonset-with-network-policies",
			obj: func() runtime.Object {
				spec, err := k8stemplates.FlannelDaemonSetTemplate(&k8s.BootstrapManifestsConfigSpec{
					FlannelImage:     "quay.io/coreos/flannel:v0.14.0",
					FlannelExtraArgs: []string{"--foo=bar"},
					FlannelResources: k8s.Resources{
						Requests: map[string]string{
							"cpu":    "100m",
							"memory": "50Mi",
						},
						Limits: map[string]string{
							"cpu":    "200m",
							"memory": "100Mi",
						},
					},
					FlannelKubeNetworkPoliciesEnabled: true,
					FlannelKubeNetworkPoliciesImage:   "registry.k8s.io/networking/kube-network-policies:v0.7.0",
				})
				require.NoError(t, err)

				return spec
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
