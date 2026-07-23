// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// These tests ensure that v1alpha1 types properly implement new-style config interfaces.

func TestKubeSchedulerBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectDisabled bool
	}{
		{
			name: "v1alpha1 only, disabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineControlPlane: &v1alpha1.MachineControlPlaneConfig{
							MachineScheduler: &v1alpha1.MachineSchedulerConfig{
								MachineSchedulerDisabled: new(true),
							},
						},
					},
				})
			},

			expectDisabled: true,
		},
		{
			name: "new style disabled",

			cfg: func(*testing.T) config.Config {
				sc := k8s.NewKubeSchedulerConfigV1Alpha1()
				sc.PodEnabled = new(false)

				c, err := container.New(
					sc,
				)
				require.NoError(t, err)

				return c
			},

			expectDisabled: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						SchedulerConfig: &v1alpha1.SchedulerConfig{
							ContainerImage: "scheduler:v1",
							ExtraArgsConfig: meta.Args{
								"features": meta.NewArgValue("all", nil),
							},
						},
					},
				})
			},
		},
		{
			name: "new style enabled",

			cfg: func(*testing.T) config.Config {
				sc := k8s.NewKubeSchedulerConfigV1Alpha1()
				sc.PodImage = "scheduler:v1"
				sc.PodArgs = meta.Args{
					"features": meta.NewArgValue("all", nil),
				}

				c, err := container.New(
					sc,
				)
				require.NoError(t, err)

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			kubeScheduler := cfg.K8sSchedulerConfig()
			require.NotNil(t, kubeScheduler)

			if test.expectDisabled {
				assert.False(t, kubeScheduler.Enabled())

				return
			}

			assert.True(t, kubeScheduler.Enabled())
			assert.Equal(t, "scheduler:v1", kubeScheduler.Image())
			assert.Equal(t, map[string][]string{"features": {"all"}}, kubeScheduler.ExtraArgs())
		})
	}
}

func TestKubeControllerManagerBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectDisabled bool
	}{
		{
			name: "v1alpha1 only, disabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineControlPlane: &v1alpha1.MachineControlPlaneConfig{
							MachineControllerManager: &v1alpha1.MachineControllerManagerConfig{
								MachineControllerManagerDisabled: new(true),
							},
						},
					},
				})
			},

			expectDisabled: true,
		},
		{
			name: "new style disabled",

			cfg: func(*testing.T) config.Config {
				cm := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cm.PodEnabled = new(false)

				c, err := container.New(
					cm,
				)
				require.NoError(t, err)

				return c
			},

			expectDisabled: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{ //nolint:staticcheck // testing deprecated field
							ContainerImage: "controller-manager:v1",
							ExtraArgsConfig: meta.Args{
								"features": meta.NewArgValue("all", nil),
							},
						},
					},
				})
			},
		},
		{
			name: "new style enabled",

			cfg: func(*testing.T) config.Config {
				cm := k8s.NewKubeControllerManagerConfigV1Alpha1()
				cm.PodImage = "controller-manager:v1"
				cm.PodArgs = meta.Args{
					"features": meta.NewArgValue("all", nil),
				}

				c, err := container.New(
					cm,
				)
				require.NoError(t, err)

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			kubeControllerManager := cfg.K8sControllerManagerConfig()
			require.NotNil(t, kubeControllerManager)

			if test.expectDisabled {
				assert.False(t, kubeControllerManager.Enabled())

				return
			}

			assert.True(t, kubeControllerManager.Enabled())
			assert.Equal(t, "controller-manager:v1", kubeControllerManager.Image())
			assert.Equal(t, map[string][]string{"features": {"all"}}, kubeControllerManager.ExtraArgs())
		})
	}
}

func TestKubeProxyBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectDisabled bool
	}{
		{
			name: "v1alpha1 only, disabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ProxyConfig: &v1alpha1.ProxyConfig{
							Disabled: new(true),
						},
					},
				})
			},

			expectDisabled: true,
		},
		{
			name: "new style disabled",

			cfg: func(*testing.T) config.Config {
				p := k8s.NewKubeProxyConfigV1Alpha1()
				p.ProxyEnabled = new(false)

				c, err := container.New(
					p,
				)
				require.NoError(t, err)

				return c
			},

			expectDisabled: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ProxyConfig: &v1alpha1.ProxyConfig{ //nolint:staticcheck // testing deprecated field
							ContainerImage: "proxy:v1",
							ModeConfig:     "ipvs",
							ExtraArgsConfig: meta.Args{
								"features": meta.NewArgValue("all", nil),
							},
						},
					},
				})
			},
		},
		{
			name: "new style enabled",

			cfg: func(*testing.T) config.Config {
				p := k8s.NewKubeProxyConfigV1Alpha1()
				p.ProxyImage = "proxy:v1"
				p.ProxyMode = "ipvs"
				p.ProxyExtraArgs = meta.Args{
					"features": meta.NewArgValue("all", nil),
				}

				c, err := container.New(
					p,
				)
				require.NoError(t, err)

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			kubeProxy := cfg.K8sProxyConfig()
			require.NotNil(t, kubeProxy)

			if test.expectDisabled {
				assert.False(t, kubeProxy.Enabled())

				return
			}

			assert.True(t, kubeProxy.Enabled())
			assert.Equal(t, "proxy:v1", kubeProxy.Image())
			assert.Equal(t, "ipvs", kubeProxy.Mode())
			assert.Equal(t, map[string][]string{"features": {"all"}}, kubeProxy.ExtraArgs())
		})
	}
}

func TestKubeCoreDNSBridge(t *testing.T) {
	t.Parallel()

	defaultImage := constants.CoreDNSImage + ":" + constants.DefaultCoreDNSVersion

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectDisabled bool
		expectImage    string
	}{
		{
			name: "v1alpha1 only, disabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						CoreDNSConfig: &v1alpha1.CoreDNS{ //nolint:staticcheck // testing deprecated field
							CoreDNSDisabled: new(true),
						},
					},
				})
			},

			expectDisabled: true,
		},
		{
			name: "new style disabled",

			cfg: func(*testing.T) config.Config {
				cd := k8s.NewKubeCoreDNSConfigV1Alpha1()
				cd.PodEnabled = new(false)

				c, err := container.New(
					cd,
				)
				require.NoError(t, err)

				return c
			},

			expectDisabled: true,
		},
		{
			name: "v1alpha1 default",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectImage: defaultImage,
		},
		{
			name: "new style default",

			cfg: func(*testing.T) config.Config {
				c, err := container.New(
					k8s.NewKubeCoreDNSConfigV1Alpha1(),
				)
				require.NoError(t, err)

				return c
			},

			expectImage: defaultImage,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						CoreDNSConfig: &v1alpha1.CoreDNS{ //nolint:staticcheck // testing deprecated field
							CoreDNSImage: "coredns:v1",
						},
					},
				})
			},

			expectImage: "coredns:v1",
		},
		{
			name: "new style enabled",

			cfg: func(*testing.T) config.Config {
				cd := k8s.NewKubeCoreDNSConfigV1Alpha1()
				cd.PodImage = "coredns:v1"

				c, err := container.New(
					cd,
				)
				require.NoError(t, err)

				return c
			},

			expectImage: "coredns:v1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			coreDNS := cfg.K8sCoreDNSConfig()
			require.NotNil(t, coreDNS)

			if test.expectDisabled {
				assert.False(t, coreDNS.Enabled())

				return
			}

			assert.True(t, coreDNS.Enabled())
			assert.Equal(t, test.expectImage, coreDNS.Image())
		})
	}
}

func TestKubeNetworkBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectHasNetworkConfig bool
		expectHasFlannelConfig bool

		expectFlannelKubeNetworkPoliciesEnabled bool
		expectPodCIDRs                          []string
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
							PodSubnet: []string{"10.0.0.0/8"},
							CNI: &v1alpha1.CNIConfig{
								CNIName: "flannel",
								CNIFlannel: &v1alpha1.FlannelCNIConfig{
									FlannelKubeNetworkPoliciesEnabled: new(true),
								},
							},
						},
					},
				})
			},

			expectHasNetworkConfig:                  true,
			expectHasFlannelConfig:                  true,
			expectFlannelKubeNetworkPoliciesEnabled: true,
			expectPodCIDRs:                          []string{"10.0.0.0/8"},
		},
		{
			name: "new style",

			cfg: func(*testing.T) config.Config {
				cn := k8s.NewKubeNetworkConfigV1Alpha1()
				cn.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
				}

				cf := k8s.NewKubeFlannelCNIConfigV1Alpha1()
				cf.FlannelKubeNetworkPoliciesEnabled = new(true)

				c, err := container.New(
					cn,
					cf,
				)
				require.NoError(t, err)

				return c
			},

			expectHasNetworkConfig:                  true,
			expectHasFlannelConfig:                  true,
			expectFlannelKubeNetworkPoliciesEnabled: true,
			expectPodCIDRs:                          []string{"10.0.0.0/8"},
		},
		{
			name: "v1alpha1 default flannel config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
							PodSubnet: []string{"10.0.0.0/8"},
						},
					},
				})
			},

			expectHasNetworkConfig:                  true,
			expectHasFlannelConfig:                  true,
			expectFlannelKubeNetworkPoliciesEnabled: false,
			expectPodCIDRs:                          []string{"10.0.0.0/8"},
		},
		{
			name: "v1alpha1 no flannel",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
							PodSubnet: []string{"10.0.0.0/8"},
							CNI: &v1alpha1.CNIConfig{
								CNIName: "custom",
							},
						},
					},
				})
			},

			expectHasNetworkConfig: true,
			expectHasFlannelConfig: false,
			expectPodCIDRs:         []string{"10.0.0.0/8"},
		},
		{
			name: "new style no flannel",

			cfg: func(*testing.T) config.Config {
				cn := k8s.NewKubeNetworkConfigV1Alpha1()
				cn.NetworkPodSubnets = []meta.Prefix{
					{Prefix: netip.MustParsePrefix("10.0.0.0/8")},
				}

				c, err := container.New(
					cn,
				)
				require.NoError(t, err)

				return c
			},

			expectHasNetworkConfig: true,
			expectHasFlannelConfig: false,
			expectPodCIDRs:         []string{"10.0.0.0/8"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			kubeNetwork := cfg.K8sNetworkConfig()

			if test.expectHasNetworkConfig {
				require.NotNil(t, kubeNetwork)
				assert.Equal(t, test.expectPodCIDRs, xslices.Map(kubeNetwork.PodCIDRs(), netip.Prefix.String))
			} else {
				require.Nil(t, kubeNetwork)
			}

			kubeFlannel := cfg.K8sFlannelCNIConfig()

			if test.expectHasFlannelConfig {
				require.NotNil(t, kubeFlannel)
				assert.Equal(t, test.expectFlannelKubeNetworkPoliciesEnabled, kubeFlannel.KubeNetworkPoliciesEnabled())
			} else {
				require.Nil(t, kubeFlannel)
			}
		})
	}
}

func TestKubeAPIServerBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectImage     string
		expectExtraArgs map[string][]string
		expectEnv       map[string]string
		expectCertSANs  []string
		expectAPIPort   int

		// the new-style config enables the authentication config file and startup probes by default,
		// while the legacy v1alpha1 config does not.
		expectStartupProbesEnabled    bool
		expectUseAuthenticationConfig bool
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						APIServerConfig: &v1alpha1.APIServerConfig{
							ContainerImage: "apiserver:v1",
							ExtraArgsConfig: meta.Args{
								"features": meta.NewArgValue("all", nil),
							},
							EnvConfig: v1alpha1.Env{
								"HTTP_PROXY": "http://proxy:8080",
							},
							ExtraCertSANs: []string{"k8s.example.com"},
						},
					},
				})
			},

			expectImage:     "apiserver:v1",
			expectExtraArgs: map[string][]string{"features": {"all"}},
			expectEnv:       map[string]string{"HTTP_PROXY": "http://proxy:8080"},
			expectCertSANs:  []string{"k8s.example.com"},
			expectAPIPort:   constants.DefaultControlPlanePort,

			expectStartupProbesEnabled:    false,
			expectUseAuthenticationConfig: false,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				as := k8s.NewKubeAPIServerConfigV1Alpha1()
				as.PodImage = "apiserver:v1"
				as.PodArgs = meta.Args{
					"features": meta.NewArgValue("all", nil),
				}
				as.PodEnv = map[string]string{
					"HTTP_PROXY": "http://proxy:8080",
				}
				as.PodCertExtraSANs = []string{"k8s.example.com"}

				c, err := container.New(as)
				require.NoError(t, err)

				return c
			},

			expectImage:     "apiserver:v1",
			expectExtraArgs: map[string][]string{"features": {"all"}},
			expectEnv:       map[string]string{"HTTP_PROXY": "http://proxy:8080"},
			expectCertSANs:  []string{"k8s.example.com"},
			expectAPIPort:   constants.DefaultControlPlanePort,

			expectStartupProbesEnabled:    true,
			expectUseAuthenticationConfig: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			apiServer := cfg.K8sAPIServerConfig()
			require.NotNil(t, apiServer)

			assert.Equal(t, test.expectImage, apiServer.Image())
			assert.Equal(t, test.expectExtraArgs, apiServer.ExtraArgs())
			assert.Equal(t, test.expectEnv, apiServer.Env())
			assert.Equal(t, test.expectCertSANs, apiServer.CertSANs())
			assert.Equal(t, test.expectAPIPort, apiServer.APIPort())
			assert.Equal(t, test.expectStartupProbesEnabled, apiServer.StartupProbesEnabled())
			assert.Equal(t, test.expectUseAuthenticationConfig, apiServer.UseAuthenticationConfig())
		})
	}
}

func TestKubeAdmissionControlBridge(t *testing.T) {
	t.Parallel()

	pluginConfiguration := map[string]any{
		"apiVersion": "pod-security.admission.config.k8s.io/v1alpha1",
		"kind":       "PodSecurityConfiguration",
		"defaults": map[string]any{
			"enforce": "baseline",
		},
	}

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						APIServerConfig: &v1alpha1.APIServerConfig{
							AdmissionControlConfig: v1alpha1.AdmissionPluginConfigList{
								{
									PluginName: "PodSecurity",
									PluginConfiguration: meta.Unstructured{
										Object: pluginConfiguration,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				ac := k8s.NewKubeAdmissionControlConfigV1Alpha1()
				ac.MetaName = "PodSecurity"
				ac.PluginConfig.Object = pluginConfiguration

				c, err := container.New(ac)
				require.NoError(t, err)

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			plugins := cfg.K8sAdmissionControlPluginConfigs()
			require.Len(t, plugins, 1)

			assert.Equal(t, "PodSecurity", plugins[0].Name())
			assert.Equal(t, pluginConfiguration, plugins[0].Configuration())
		})
	}
}

func TestKubeAuditPolicyBridge(t *testing.T) {
	t.Parallel()

	customPolicy := map[string]any{
		"apiVersion": "audit.k8s.io/v1",
		"kind":       "Policy",
		"rules": []any{
			map[string]any{
				"level": "RequestResponse",
			},
		},
	}

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectConfiguration map[string]any
	}{
		{
			name: "v1alpha1 default",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						APIServerConfig: &v1alpha1.APIServerConfig{},
					},
				})
			},

			// the legacy config falls back to a built-in default audit policy when none is set.
			expectConfiguration: v1alpha1.APIServerDefaultAuditPolicy.Object,
		},
		{
			name: "v1alpha1 custom",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						APIServerConfig: &v1alpha1.APIServerConfig{
							AuditPolicyConfig: meta.Unstructured{
								Object: customPolicy,
							},
						},
					},
				})
			},

			expectConfiguration: customPolicy,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				ap := k8s.NewKubeAuditPolicyConfigV1Alpha1()
				ap.AuditConfig.Object = customPolicy

				c, err := container.New(ap)
				require.NoError(t, err)

				return c
			},

			expectConfiguration: customPolicy,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			auditPolicy := cfg.K8sAuditPolicyConfig()
			require.NotNil(t, auditPolicy)

			assert.Equal(t, test.expectConfiguration, auditPolicy.Configuration())
		})
	}
}

func TestKubeAuthorizerBridge(t *testing.T) {
	t.Parallel()

	webhook := map[string]any{
		"timeout":                    "3s",
		"subjectAccessReviewVersion": "v1",
	}

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						APIServerConfig: &v1alpha1.APIServerConfig{
							AuthorizationConfigConfig: v1alpha1.AuthorizationConfigAuthorizerConfigList{
								{
									AuthorizerType: "Node",
									AuthorizerName: "node",
								},
								{
									AuthorizerType: "Webhook",
									AuthorizerName: "webhook",
									AuthorizerWebhook: meta.Unstructured{
										Object: webhook,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				node := k8s.NewKubeAuthorizerConfigV1Alpha1()
				node.MetaName = "node"
				node.AuthorizerType = "Node"

				wh := k8s.NewKubeAuthorizerConfigV1Alpha1()
				wh.MetaName = "webhook"
				wh.AuthorizerType = "Webhook"
				wh.AuthorizerWebhook.Object = webhook

				c, err := container.New(node, wh)
				require.NoError(t, err)

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			authorizers := cfg.K8sAuthorizerConfigs()
			require.Len(t, authorizers, 2)

			assert.Equal(t, "node", authorizers[0].Name())
			assert.Equal(t, "Node", authorizers[0].Type())
			assert.Empty(t, authorizers[0].Webhook())

			assert.Equal(t, "webhook", authorizers[1].Name())
			assert.Equal(t, "Webhook", authorizers[1].Type())
			assert.Equal(t, webhook, authorizers[1].Webhook())
		})
	}
}

func TestKubeAPIServerCABridge(t *testing.T) {
	t.Parallel()

	// the issuing CA (with a private key) is only present on the controlplane.
	issuingCA, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	// an additional accepted (rotated-out) CA, certificate only.
	acceptedCA, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectIssuingCA   *x509.PEMEncodedCertificateAndKey
		expectAcceptedCAs []*x509.PEMEncodedCertificate
	}{
		{
			// worker-style config: the machine only has the CA certificate, no private key,
			// so it can't issue certificates, but trusts the CA.
			name: "v1alpha1 worker",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterCA: &x509.PEMEncodedCertificateAndKey{
							Crt: issuingCA.CrtPEM,
						},
					},
				})
			},

			expectIssuingCA: nil,
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: issuingCA.CrtPEM},
			},
		},
		{
			name: "new style worker",

			cfg: func(t *testing.T) config.Config {
				ca := k8s.NewKubeAPIServerCAConfigV1Alpha1()
				ca.APIAcceptedCAs = []string{string(issuingCA.CrtPEM)}

				c, err := container.New(ca)
				require.NoError(t, err)

				return c
			},

			expectIssuingCA: nil,
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: issuingCA.CrtPEM},
			},
		},
		{
			// controlplane-style config: full PKI, the machine has the issuing CA key pair and
			// an additional accepted CA certificate.
			name: "v1alpha1 controlplane",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterCA: &x509.PEMEncodedCertificateAndKey{
							Crt: issuingCA.CrtPEM,
							Key: issuingCA.KeyPEM,
						},
						ClusterAcceptedCAs: []*x509.PEMEncodedCertificate{
							{Crt: acceptedCA.CrtPEM},
						},
					},
				})
			},

			expectIssuingCA: &x509.PEMEncodedCertificateAndKey{
				Crt: issuingCA.CrtPEM,
				Key: issuingCA.KeyPEM,
			},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				// the issuing CA certificate is prepended to the list of accepted CAs.
				{Crt: issuingCA.CrtPEM},
				{Crt: acceptedCA.CrtPEM},
			},
		},
		{
			name: "new style controlplane",

			cfg: func(t *testing.T) config.Config {
				ca := k8s.NewKubeAPIServerCAConfigV1Alpha1()
				ca.APIIssuingCA = &meta.CertificateAndKey{
					Cert: string(issuingCA.CrtPEM),
					Key:  string(issuingCA.KeyPEM),
				}
				ca.APIAcceptedCAs = []string{string(acceptedCA.CrtPEM)}

				c, err := container.New(ca)
				require.NoError(t, err)

				return c
			},

			expectIssuingCA: &x509.PEMEncodedCertificateAndKey{
				Crt: issuingCA.CrtPEM,
				Key: issuingCA.KeyPEM,
			},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				// the issuing CA certificate is prepended to the list of accepted CAs.
				{Crt: issuingCA.CrtPEM},
				{Crt: acceptedCA.CrtPEM},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			apiServerCA := cfg.K8sAPIServerCAConfig()
			require.NotNil(t, apiServerCA)

			assert.Equal(t, test.expectIssuingCA, apiServerCA.IssuingCA())
			assert.Equal(t, test.expectAcceptedCAs, apiServerCA.AcceptedCAs())
		})
	}
}

func TestKubeServiceAccountBridge(t *testing.T) {
	t.Parallel()

	// the service account issuing key is only present on the controlplane.
	sa, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	issuingKey, err := (&x509.PEMEncodedKey{Key: sa.KeyPEM}).GetKey()
	require.NoError(t, err)

	endpoint := ensure.Value(url.Parse("https://cluster-endpoint:6443"))

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil          bool
		expectIssuerURL    string
		expectIssuingKey   *x509.PEMEncodedKey
		expectAcceptedKeys []*x509.PEMEncodedKey
	}{
		{
			name: "no cluster config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "cluster config without service account",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 with service account",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ControlPlane: &v1alpha1.ControlPlaneConfig{
							Endpoint: &v1alpha1.Endpoint{URL: endpoint},
						},
						ClusterServiceAccount: &x509.PEMEncodedKey{
							Key: sa.KeyPEM,
						},
					},
				})
			},

			expectIssuerURL:  "https://cluster-endpoint:6443",
			expectIssuingKey: &x509.PEMEncodedKey{Key: sa.KeyPEM},
			// the accepted keys are derived by exposing the public key of the issuing key.
			expectAcceptedKeys: []*x509.PEMEncodedKey{
				{Key: issuingKey.GetPublicKeyPEM()},
			},
		},
		{
			// the new-style multi-doc config, set up to be equivalent to the legacy config above:
			// the same issuing key and endpoint, with no additional accepted keys/issuers/audiences.
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				serviceAccount := k8s.NewKubeServiceAccountConfigV1Alpha1()
				serviceAccount.ServiceIssuer = k8s.IssuerServiceAccountConfig{
					PrivateKey: string(sa.KeyPEM),
					IssuerURL:  meta.URL{URL: endpoint},
				}

				c, err := container.New(serviceAccount)
				require.NoError(t, err)

				return c
			},

			expectIssuerURL:  "https://cluster-endpoint:6443",
			expectIssuingKey: &x509.PEMEncodedKey{Key: sa.KeyPEM},
			expectAcceptedKeys: []*x509.PEMEncodedKey{
				{Key: issuingKey.GetPublicKeyPEM()},
			},
		},
		{
			// a broken key is tolerated for backwards compatibility: the actual failure was
			// previously deferred to the controllers, so the accepted keys come back empty.
			name: "v1alpha1 with broken service account key",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ControlPlane: &v1alpha1.ControlPlaneConfig{
							Endpoint: &v1alpha1.Endpoint{URL: endpoint},
						},
						ClusterServiceAccount: &x509.PEMEncodedKey{
							Key: []byte("not a valid key"),
						},
					},
				})
			},

			expectIssuerURL:    "https://cluster-endpoint:6443",
			expectIssuingKey:   &x509.PEMEncodedKey{Key: []byte("not a valid key")},
			expectAcceptedKeys: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			serviceAccount := cfg.K8sServiceAccountConfig()

			if test.expectNil {
				assert.Nil(t, serviceAccount)

				return
			}

			require.NotNil(t, serviceAccount)

			assert.Equal(t, test.expectIssuerURL, serviceAccount.IssuerURL())
			assert.Equal(t, test.expectIssuingKey, serviceAccount.IssuingKey())
			assert.Equal(t, test.expectAcceptedKeys, serviceAccount.AcceptedKeys())

			// the legacy config doesn't support additional accepted issuers.
			assert.Nil(t, serviceAccount.AcceptedIssuers())

			// the API audiences default to the issuer URL.
			assert.Equal(t, []string{test.expectIssuerURL}, serviceAccount.APIAudiences())
		})
	}
}

func TestKubeAggregatorCABridge(t *testing.T) {
	t.Parallel()

	aggregatorCA, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectIssuingCA   *x509.PEMEncodedCertificateAndKey
		expectAcceptedCAs []*x509.PEMEncodedCertificate
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterAggregatorCA: &x509.PEMEncodedCertificateAndKey{ //nolint:staticcheck // testing deprecated field
							Crt: aggregatorCA.CrtPEM,
							Key: aggregatorCA.KeyPEM,
						},
					},
				})
			},

			// the issuing CA exposes the full key pair, while only its certificate is exposed as an accepted CA.
			expectIssuingCA: &x509.PEMEncodedCertificateAndKey{
				Crt: aggregatorCA.CrtPEM,
				Key: aggregatorCA.KeyPEM,
			},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: aggregatorCA.CrtPEM},
			},
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				ca := k8s.NewKubeAggregatorCAConfigV1Alpha1()
				ca.AggregatorIssuingCA = &meta.CertificateAndKey{
					Cert: string(aggregatorCA.CrtPEM),
					Key:  string(aggregatorCA.KeyPEM),
				}

				c, err := container.New(ca)
				require.NoError(t, err)

				return c
			},

			expectIssuingCA: &x509.PEMEncodedCertificateAndKey{
				Crt: aggregatorCA.CrtPEM,
				Key: aggregatorCA.KeyPEM,
			},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				// the issuing CA certificate is prepended to the list of accepted CAs.
				{Crt: aggregatorCA.CrtPEM},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			aggregatorCAConfig := cfg.K8sAggregatorCAConfig()
			require.NotNil(t, aggregatorCAConfig)

			assert.Equal(t, test.expectIssuingCA, aggregatorCAConfig.IssuingCA())
			assert.Equal(t, test.expectAcceptedCAs, aggregatorCAConfig.AcceptedCAs())
		})
	}
}

func TestKubeNodeConfigBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectSkipNodeRegistration bool
		expectRegisterWithFQDN     bool
		expectValidSubnets         []string
		expectLabels               map[string]string
		expectAnnotations          map[string]string
		expectTaints               map[string]string
	}{
		{
			name: "no machine or cluster config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 controlplane",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineType: "controlplane",
						MachineKubelet: &v1alpha1.KubeletConfig{
							KubeletSkipNodeRegistration: new(true),
							KubeletRegisterWithFQDN:     new(true),
							KubeletNodeIP: &v1alpha1.KubeletNodeIPConfig{
								KubeletNodeIPValidSubnets: []string{"10.0.0.0/8"},
							},
						},
						MachineNodeLabels:      map[string]string{"examplelabel": "examplevalue"},
						MachineNodeAnnotations: map[string]string{"customer.io/rack": "r13a25"},
						MachineNodeTaints:      map[string]string{"exampletaint": "examplevalue:NoSchedule"},
					},
					ClusterConfig: &v1alpha1.ClusterConfig{},
				})
			},

			expectSkipNodeRegistration: true,
			expectRegisterWithFQDN:     true,
			expectValidSubnets:         []string{"10.0.0.0/8"},
			// the control plane role label is injected for control plane nodes.
			expectLabels: map[string]string{
				"examplelabel":                      "examplevalue",
				constants.LabelNodeRoleControlPlane: "",
			},
			expectAnnotations: map[string]string{"customer.io/rack": "r13a25"},
			// the control plane taint is injected when scheduling on control planes is not allowed.
			expectTaints: map[string]string{
				"exampletaint":                      "examplevalue:NoSchedule",
				constants.LabelNodeRoleControlPlane: constants.TaintEffectNoSchedule,
			},
		},
		{
			name: "v1alpha1 worker with scheduling allowed",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineType:       "worker",
						MachineNodeLabels: map[string]string{"examplelabel": "examplevalue"},
					},
				})
			},

			// no control plane role label for worker nodes, and no control plane taint when scheduling is allowed.
			expectLabels: map[string]string{"examplelabel": "examplevalue"},
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				nc := k8s.NewKubeNodeConfigV1Alpha1()
				nc.SkipNodeRegistrationConfig = new(true)
				nc.RegisterWithFQDNConfig = new(true)
				nc.NodeIPConfig = k8s.NodeIPConfig{
					NodeIPValidSubnets: []string{"10.0.0.0/8"},
				}
				nc.LabelsConfig = map[string]string{"examplelabel": "examplevalue"}
				nc.AnnotationsConfig = map[string]string{"customer.io/rack": "r13a25"}
				nc.TaintsConfig = map[string]string{"exampletaint": "examplevalue:NoSchedule"}

				c, err := container.New(nc)
				require.NoError(t, err)

				return c
			},

			// the new-style config exposes the fields verbatim, without injecting control plane labels/taints.
			expectSkipNodeRegistration: true,
			expectRegisterWithFQDN:     true,
			expectValidSubnets:         []string{"10.0.0.0/8"},
			expectLabels:               map[string]string{"examplelabel": "examplevalue"},
			expectAnnotations:          map[string]string{"customer.io/rack": "r13a25"},
			expectTaints:               map[string]string{"exampletaint": "examplevalue:NoSchedule"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			nodeConfig := cfg.K8sNodeConfig()

			if test.expectNil {
				assert.Nil(t, nodeConfig)

				return
			}

			require.NotNil(t, nodeConfig)

			assert.Equal(t, test.expectSkipNodeRegistration, nodeConfig.SkipNodeRegistration())
			assert.Equal(t, test.expectRegisterWithFQDN, nodeConfig.RegisterWithFQDN())
			assert.Equal(t, test.expectValidSubnets, nodeConfig.NodeIP().ValidSubnets())
			assert.Equal(t, test.expectLabels, nodeConfig.Labels())
			assert.Equal(t, test.expectAnnotations, nodeConfig.Annotations())
			assert.Equal(t, test.expectTaints, nodeConfig.Taints())
		})
	}
}

func TestKubeKubeletConfigBridge(t *testing.T) {
	t.Parallel()

	defaultImage := constants.KubeletImage + ":v" + constants.DefaultKubernetesVersion

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectImage                     string
		expectClusterDNS                []string
		expectExtraArgs                 map[string][]string
		expectExtraConfig               map[string]any
		expectSeccompEnabled            bool
		expectDisableManifestsDirectory bool
	}{
		{
			name: "no machine config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 default kubelet",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			// the legacy config falls back to the default kubelet image when none is set.
			expectImage:     defaultImage,
			expectExtraArgs: map[string][]string{},
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineKubelet: &v1alpha1.KubeletConfig{
							KubeletImage:      "kubelet:v1",
							KubeletClusterDNS: []string{"10.96.0.10"},
							KubeletExtraArgs: meta.Args{
								"feature-gates": meta.NewArgValue("AllBeta=true", nil),
							},
							KubeletExtraConfig: meta.Unstructured{
								Object: map[string]any{"serverTLSBootstrap": true},
							},
							KubeletDefaultRuntimeSeccompProfileEnabled: new(true),
							KubeletDisableManifestsDirectory:           new(true),
						},
					},
				})
			},

			expectImage:                     "kubelet:v1",
			expectClusterDNS:                []string{"10.96.0.10"},
			expectExtraArgs:                 map[string][]string{"feature-gates": {"AllBeta=true"}},
			expectExtraConfig:               map[string]any{"serverTLSBootstrap": true},
			expectSeccompEnabled:            true,
			expectDisableManifestsDirectory: true,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				kc := k8s.NewKubeletConfigV1Alpha1()
				kc.KubeletImage = "kubelet:v1"
				kc.KubeletClusterDNS = []string{"10.96.0.10"}
				kc.KubeletArgs = meta.Args{
					"feature-gates": meta.NewArgValue("AllBeta=true", nil),
				}
				kc.KubeletConfig = meta.Unstructured{
					Object: map[string]any{"serverTLSBootstrap": true},
				}
				kc.KubeletDefaultRuntimeSeccompProfileEnabled = new(true)

				c, err := container.New(kc)
				require.NoError(t, err)

				return c
			},

			expectImage:          "kubelet:v1",
			expectClusterDNS:     []string{"10.96.0.10"},
			expectExtraArgs:      map[string][]string{"feature-gates": {"AllBeta=true"}},
			expectExtraConfig:    map[string]any{"serverTLSBootstrap": true},
			expectSeccompEnabled: true,
			// the new-style config always disables the static manifests directory.
			expectDisableManifestsDirectory: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			kubelet := cfg.K8sKubeletConfig()

			if test.expectNil {
				assert.Nil(t, kubelet)

				return
			}

			require.NotNil(t, kubelet)

			assert.Equal(t, test.expectImage, kubelet.Image())
			assert.Equal(t, test.expectClusterDNS, kubelet.ClusterDNS())
			assert.Equal(t, test.expectExtraArgs, kubelet.ExtraArgs())
			assert.Equal(t, test.expectExtraConfig, kubelet.ExtraConfig())
			assert.Equal(t, test.expectSeccompEnabled, kubelet.DefaultRuntimeSeccompProfileEnabled())
			assert.Equal(t, test.expectDisableManifestsDirectory, kubelet.DisableManifestsDirectory())
			assert.Nil(t, kubelet.ExtraMounts())
		})
	}
}

func TestKubeCredentialProviderConfigBridge(t *testing.T) {
	t.Parallel()

	credentialProviderConfig := map[string]any{
		"apiVersion": "kubelet.config.k8s.io/v1",
		"kind":       "CredentialProviderConfig",
		"providers": []any{
			map[string]any{
				"name":       "ecr-credential-provider",
				"apiVersion": "credentialprovider.kubelet.k8s.io/v1",
				"matchImages": []any{
					"*.dkr.ecr.*.amazonaws.com",
				},
				"defaultCacheDuration": "12h",
			},
		},
	}

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectConfiguration map[string]any
	}{
		{
			name: "no machine config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "machine config without kubelet",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineKubelet: &v1alpha1.KubeletConfig{
							KubeletCredentialProviderConfig: meta.Unstructured{
								Object: credentialProviderConfig,
							},
						},
					},
				})
			},

			expectConfiguration: credentialProviderConfig,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				cp := k8s.NewKubeCredentialProviderConfigV1Alpha1()
				cp.CredentialProviderConfig.Object = credentialProviderConfig

				c, err := container.New(cp)
				require.NoError(t, err)

				return c
			},

			expectConfiguration: credentialProviderConfig,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			credentialProvider := cfg.K8sCredentialProviderConfig()

			if test.expectNil {
				assert.Nil(t, credentialProvider)

				return
			}

			require.NotNil(t, credentialProvider)

			assert.Equal(t, test.expectConfiguration, credentialProvider.Configuration())
		})
	}
}

func TestKubeStaticPodConfigBridge(t *testing.T) {
	t.Parallel()

	podSpec := map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      "nginx",
			"namespace": "ci",
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx",
				},
			},
		},
	}

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectName string
		expectPod  map[string]any
	}{
		{
			name: "no machine config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachinePods: []meta.Unstructured{
							{Object: podSpec},
						},
					},
				})
			},

			// the legacy config has no name, so it's synthesized from the pod's namespace and name.
			expectName: "ci-nginx",
			expectPod:  podSpec,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				sp := k8s.NewKubeStaticPodConfigV1Alpha1()
				sp.MetaName = "nginx"
				sp.PodSpec.Object = podSpec

				c, err := container.New(sp)
				require.NoError(t, err)

				return c
			},

			expectName: "nginx",
			expectPod:  podSpec,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			staticPods := cfg.K8sStaticPodConfigs()

			if test.expectNil {
				assert.Nil(t, staticPods)

				return
			}

			require.Len(t, staticPods, 1)

			assert.Equal(t, test.expectName, staticPods[0].Name())
			assert.Equal(t, test.expectPod, staticPods[0].Pod())
		})
	}
}

func TestKubeInlineManifestConfigBridge(t *testing.T) {
	t.Parallel()

	contents := strings.TrimSpace(`
apiVersion: v1
kind: Namespace
metadata:
  name: ci
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: build-settings
  namespace: ci
data:
  parallelism: "4"
`)

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectName     string
		expectContents string
	}{
		{
			name: "no cluster config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterInlineManifests: v1alpha1.ClusterInlineManifests{
							{
								InlineManifestName:     "namespace-ci",
								InlineManifestContents: contents,
							},
						},
					},
				})
			},

			expectName:     "namespace-ci",
			expectContents: contents,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				im := k8s.NewKubeInlineManifestConfigV1Alpha1()
				im.MetaName = "namespace-ci"
				im.ManifestSpec = contents

				c, err := container.New(im)
				require.NoError(t, err)

				return c
			},

			expectName:     "namespace-ci",
			expectContents: contents,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			manifests := cfg.K8sInlineManifestConfigs()

			if test.expectNil {
				assert.Nil(t, manifests)

				return
			}

			require.Len(t, manifests, 1)

			assert.Equal(t, test.expectName, manifests[0].Name())
			assert.Equal(t, test.expectContents, manifests[0].Contents())
		})
	}
}

func TestKubeExternalManifestConfigBridge(t *testing.T) {
	t.Parallel()

	manifestURL := "https://www.example.com/manifest1.yaml"
	headers := map[string]string{
		"Authorization": "Bearer token",
	}

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectName    string
		expectURL     string
		expectHeaders map[string]string
	}{
		{
			name: "no cluster config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ExtraManifests:       []string{manifestURL},
						ExtraManifestHeaders: headers,
					},
				})
			},

			// the legacy config has no name, so the URL is reused as the name.
			expectName:    manifestURL,
			expectURL:     manifestURL,
			expectHeaders: headers,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				em := k8s.NewKubeExternalManifestConfigV1Alpha1()
				em.MetaName = "example-cni"
				em.HeadersSpec = headers
				em.URLSpec = meta.URL{URL: ensure.Value(url.Parse(manifestURL))}

				c, err := container.New(em)
				require.NoError(t, err)

				return c
			},

			expectName:    "example-cni",
			expectURL:     manifestURL,
			expectHeaders: headers,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			manifests := cfg.K8sExternalManifestConfigs()

			if test.expectNil {
				assert.Nil(t, manifests)

				return
			}

			require.Len(t, manifests, 1)

			assert.Equal(t, test.expectName, manifests[0].Name())
			assert.Equal(t, test.expectURL, manifests[0].URL())
			assert.Equal(t, test.expectHeaders, manifests[0].Headers())
		})
	}
}

func TestKubeClusterConfigBridge(t *testing.T) {
	t.Parallel()

	endpoint := ensure.Value(url.Parse("https://cluster-endpoint:6443/"))

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectClusterName     string
		expectClusterEndpoint *url.URL
	}{
		{
			name: "no cluster config",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					ClusterConfig: &v1alpha1.ClusterConfig{
						ClusterName: "test-cluster",
						ControlPlane: &v1alpha1.ControlPlaneConfig{
							Endpoint: &v1alpha1.Endpoint{URL: endpoint},
						},
					},
				})
			},

			expectClusterName:     "test-cluster",
			expectClusterEndpoint: endpoint,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				cc := k8s.NewKubeClusterConfigV1Alpha1()
				cc.ClusterNameConfig = "test-cluster"
				cc.ClusterEndpointConfig = meta.URL{URL: endpoint}

				c, err := container.New(cc)
				require.NoError(t, err)

				return c
			},

			expectClusterName:     "test-cluster",
			expectClusterEndpoint: endpoint,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			clusterConfig := cfg.K8sClusterConfig()

			if test.expectNil {
				assert.Nil(t, clusterConfig)

				return
			}

			require.NotNil(t, clusterConfig)

			assert.Equal(t, test.expectClusterName, clusterConfig.ClusterName())
			assert.Equal(t, test.expectClusterEndpoint, clusterConfig.ClusterEndpoint())
		})
	}
}

func TestKubePrismConfigBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil           bool
		expectPort          int
		expectTLSServerName string
	}{
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 without features",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 without KubePrism",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{},
					},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 KubePrism disabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							KubePrismSupport: &v1alpha1.KubePrism{ //nolint:staticcheck // testing deprecated field
								ServerEnabled: new(false),
								ServerPort:    8443,
							},
						},
					},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 KubePrism enabled, default port",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							KubePrismSupport: &v1alpha1.KubePrism{ //nolint:staticcheck // testing deprecated field
								ServerEnabled: new(true),
							},
						},
					},
				})
			},

			expectPort: constants.DefaultKubePrismPort,
		},
		{
			name: "v1alpha1 KubePrism enabled, explicit port",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							KubePrismSupport: &v1alpha1.KubePrism{ //nolint:staticcheck // testing deprecated field
								ServerEnabled: new(true),
								ServerPort:    8443,
							},
						},
					},
				})
			},

			expectPort: 8443,
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				kp := k8s.NewKubePrismConfigV1Alpha1()
				kp.PortConfig = 8443
				kp.TLSServerNameConfig = "api.cluster.local"

				c, err := container.New(kp)
				require.NoError(t, err)

				return c
			},

			expectPort:          8443,
			expectTLSServerName: "api.cluster.local",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			kubePrism := cfg.K8sKubePrismConfig()

			if test.expectNil {
				assert.Nil(t, kubePrism)

				return
			}

			require.NotNil(t, kubePrism)

			assert.Equal(t, test.expectPort, kubePrism.Port())
			assert.Equal(t, test.expectTLSServerName, kubePrism.TLSServerName())
		})
	}
}

func TestKubeTalosAPIAccessConfigBridge(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectNil bool

		expectAllowedRoles      []string
		expectAllowedNamespaces []string
	}{
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 without features",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 without Talos API access",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{},
					},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 Talos API access disabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{ //nolint:staticcheck // testing deprecated field
								AccessEnabled:                     new(false),
								AccessAllowedRoles:                []string{string(role.Reader)},
								AccessAllowedKubernetesNamespaces: []string{"kube-system"},
							},
						},
					},
				})
			},

			expectNil: true,
		},
		{
			name: "v1alpha1 Talos API access enabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{ //nolint:staticcheck // testing deprecated field
								AccessEnabled:                     new(true),
								AccessAllowedRoles:                []string{string(role.Reader), string(role.Operator)},
								AccessAllowedKubernetesNamespaces: []string{"kube-system", "talos-system"},
							},
						},
					},
				})
			},

			expectAllowedRoles:      []string{"os:reader", "os:operator"},
			expectAllowedNamespaces: []string{"kube-system", "talos-system"},
		},
		{
			name: "v1alpha1 Talos API access enabled, no roles",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{ //nolint:staticcheck // testing deprecated field
								AccessEnabled: new(true),
							},
						},
					},
				})
			},
		},
		{
			name: "new style",

			cfg: func(t *testing.T) config.Config {
				ta := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()
				ta.AccessAllowedRoles = []string{string(role.Reader)}
				ta.AccessAllowedKubernetesNamespaces = []string{"kube-system"}

				c, err := container.New(ta)
				require.NoError(t, err)

				return c
			},

			expectAllowedRoles:      []string{"os:reader"},
			expectAllowedNamespaces: []string{"kube-system"},
		},
		{
			name: "new style takes precedence",

			cfg: func(t *testing.T) config.Config {
				ta := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()
				ta.AccessAllowedRoles = []string{string(role.Reader)}
				ta.AccessAllowedKubernetesNamespaces = []string{"kube-system"}

				c, err := container.New(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{ //nolint:staticcheck // testing deprecated field
								AccessEnabled:                     new(true),
								AccessAllowedRoles:                []string{string(role.Admin)},
								AccessAllowedKubernetesNamespaces: []string{"default"},
							},
						},
					},
				}, ta)
				require.NoError(t, err)

				return c
			},

			expectAllowedRoles:      []string{"os:reader"},
			expectAllowedNamespaces: []string{"kube-system"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			talosAPIAccess := cfg.K8sTalosAPIAccessConfig()

			if test.expectNil {
				assert.Nil(t, talosAPIAccess)

				return
			}

			require.NotNil(t, talosAPIAccess)

			assert.Equal(t, test.expectAllowedRoles, talosAPIAccess.AllowedRoles())
			assert.Equal(t, test.expectAllowedNamespaces, talosAPIAccess.AllowedKubernetesNamespaces())
		})
	}
}
