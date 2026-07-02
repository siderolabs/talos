// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"net/netip"
	"net/url"
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
