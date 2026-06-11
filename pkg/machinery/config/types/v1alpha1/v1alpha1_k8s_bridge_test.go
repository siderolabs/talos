// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"net/netip"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
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
