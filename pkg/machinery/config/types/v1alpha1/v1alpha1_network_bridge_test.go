// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// These tests ensure that v1alpha1 types properly implement new-style config interfaces.

func TestStaticHostsBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineNetwork: &v1alpha1.NetworkConfig{
							ExtraHostEntries: []*v1alpha1.ExtraHost{
								{
									HostIP: "10.5.0.2",
									HostAliases: []string{
										"example.com",
										"example",
									},
								},
								{
									HostIP: "10.5.0.3",
									HostAliases: []string{
										"my-machine",
									},
								},
								{
									HostIP: "2001:db8::1",
									HostAliases: []string{
										"ipv6-host",
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "new style only",

			cfg: func(*testing.T) config.Config {
				host1 := network.NewStaticHostConfigV1Alpha1("10.5.0.2")
				host1.Hostnames = []string{"example.com", "example"}

				host2 := network.NewStaticHostConfigV1Alpha1("10.5.0.3")
				host2.Hostnames = []string{"my-machine"}

				host3 := network.NewStaticHostConfigV1Alpha1("2001:db8::1")
				host3.Hostnames = []string{"ipv6-host"}

				c, err := container.New(
					host1,
					host2,
					host3,
				)
				require.NoError(t, err)

				return c
			},
		},
		{
			name: "mixed",

			cfg: func(*testing.T) config.Config {
				host2 := network.NewStaticHostConfigV1Alpha1("10.5.0.3")
				host2.Hostnames = []string{"my-machine"}

				host3 := network.NewStaticHostConfigV1Alpha1("2001:db8::1")
				host3.Hostnames = []string{"ipv6-host"}

				c, err := container.New(
					host2,
					host3,
					&v1alpha1.Config{
						MachineConfig: &v1alpha1.MachineConfig{
							MachineNetwork: &v1alpha1.NetworkConfig{
								ExtraHostEntries: []*v1alpha1.ExtraHost{
									{
										HostIP: "10.5.0.2",
										HostAliases: []string{
											"example.com",
											"example",
										},
									},
								},
							},
						},
					},
				)
				require.NoError(t, err)

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			staticHosts := cfg.NetworkStaticHostConfig()

			require.Len(t, staticHosts, 3)

			assert.Equal(t, "10.5.0.2", staticHosts[0].IP())
			assert.Equal(t, []string{"example.com", "example"}, staticHosts[0].Aliases())

			assert.Equal(t, "10.5.0.3", staticHosts[1].IP())
			assert.Equal(t, []string{"my-machine"}, staticHosts[1].Aliases())

			assert.Equal(t, "2001:db8::1", staticHosts[2].IP())
			assert.Equal(t, []string{"ipv6-host"}, staticHosts[2].Aliases())
		})
	}
}

func TestHostnameBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectedHostname     string
		expectedAutoHostname nethelpers.AutoHostnameKind
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineNetwork: &v1alpha1.NetworkConfig{
							NetworkHostname: "my-machine",
						},
						MachineFeatures: &v1alpha1.FeaturesConfig{
							StableHostname: new(true),
						},
					},
				})
			},

			expectedHostname:     "my-machine",
			expectedAutoHostname: nethelpers.AutoHostnameKindStable,
		},
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectedHostname:     "",
			expectedAutoHostname: nethelpers.AutoHostnameKindAddr,
		},
		{
			name: "new style only",

			cfg: func(*testing.T) config.Config {
				hc := network.NewHostnameConfigV1Alpha1()
				hc.ConfigHostname = "my-machine"

				c, err := container.New(
					hc,
				)
				require.NoError(t, err)

				return c
			},

			expectedHostname:     "my-machine",
			expectedAutoHostname: nethelpers.AutoHostnameKindOff,
		},
		{
			name: "mixed",

			cfg: func(*testing.T) config.Config {
				hc := network.NewHostnameConfigV1Alpha1()
				hc.ConfigAuto = new(nethelpers.AutoHostnameKindStable)

				c, err := container.New(
					hc,
					&v1alpha1.Config{
						MachineConfig: &v1alpha1.MachineConfig{
							MachineNetwork: &v1alpha1.NetworkConfig{},
						},
					},
				)
				require.NoError(t, err)

				return c
			},

			expectedHostname:     "",
			expectedAutoHostname: nethelpers.AutoHostnameKindStable,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			hostnameConfig := cfg.NetworkHostnameConfig()

			require.NotNil(t, hostnameConfig)

			assert.Equal(t, test.expectedHostname, hostnameConfig.Hostname())
			assert.Equal(t, test.expectedAutoHostname, hostnameConfig.AutoHostname())
		})
	}
}

func TestResolverBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectedNameservers   []netip.Addr
		expectedSearchDomains []string
		expectedDisableSearch bool
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineNetwork: &v1alpha1.NetworkConfig{
							NameServers:                []string{"2.2.2.2", "3.3.3.3"},
							Searches:                   []string{"universe.com", "galaxy.org"},
							NetworkDisableSearchDomain: new(true),
						},
					},
				})
			},

			expectedNameservers:   []netip.Addr{netip.MustParseAddr("2.2.2.2"), netip.MustParseAddr("3.3.3.3")},
			expectedSearchDomains: []string{"universe.com", "galaxy.org"},
			expectedDisableSearch: true,
		},
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectedNameservers:   nil,
			expectedSearchDomains: nil,
			expectedDisableSearch: false,
		},
		{
			name: "new style only",

			cfg: func(*testing.T) config.Config {
				rc := network.NewResolverConfigV1Alpha1()
				rc.ResolverNameservers = []network.NameserverConfig{
					{
						Address: network.Addr{Addr: netip.MustParseAddr("2.2.2.2")},
					},
					{
						Address: network.Addr{Addr: netip.MustParseAddr("3.3.3.3")},
					},
				}
				rc.ResolverSearchDomains = network.SearchDomainsConfig{
					SearchDomains:        []string{"universe.com", "galaxy.org"},
					SearchDisableDefault: new(true),
				}

				c, err := container.New(
					rc,
				)
				require.NoError(t, err)

				return c
			},

			expectedNameservers:   []netip.Addr{netip.MustParseAddr("2.2.2.2"), netip.MustParseAddr("3.3.3.3")},
			expectedSearchDomains: []string{"universe.com", "galaxy.org"},
			expectedDisableSearch: true,
		},
		{
			name: "mixed",

			cfg: func(*testing.T) config.Config {
				rc := network.NewResolverConfigV1Alpha1()
				rc.ResolverNameservers = []network.NameserverConfig{
					{
						Address: network.Addr{Addr: netip.MustParseAddr("2.2.2.2")},
					},
					{
						Address: network.Addr{Addr: netip.MustParseAddr("3.3.3.3")},
					},
				}

				c, err := container.New(
					rc,
					&v1alpha1.Config{
						MachineConfig: &v1alpha1.MachineConfig{
							MachineNetwork: &v1alpha1.NetworkConfig{},
						},
					},
				)
				require.NoError(t, err)

				return c
			},

			expectedNameservers:   []netip.Addr{netip.MustParseAddr("2.2.2.2"), netip.MustParseAddr("3.3.3.3")},
			expectedSearchDomains: nil,
			expectedDisableSearch: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			resolverConfig := cfg.NetworkResolverConfig()

			require.NotNil(t, resolverConfig)

			assert.Equal(t, xslices.Map(test.expectedNameservers, func(addr netip.Addr) config.NetworkResolver {
				return config.NetworkResolver{
					Addr:     addr,
					Protocol: nethelpers.DNSProtocolDefault,
				}
			}), resolverConfig.Resolvers())
			assert.Equal(t, test.expectedSearchDomains, resolverConfig.SearchDomains())
			assert.Equal(t, test.expectedDisableSearch, resolverConfig.DisableSearchDomain())
		})
	}
}

func TestTimeSyncBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectedNil         bool
		expectedDisabled    bool
		expectedTimeservers []string
		expectedBootTimeout time.Duration
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineTime: &v1alpha1.TimeConfig{
							TimeDisabled:    new(true),
							TimeServers:     []string{"time1.example.com", "time2.example.com"},
							TimeBootTimeout: 30 * time.Second,
						},
					},
				})
			},

			expectedDisabled:    true,
			expectedTimeservers: []string{"time1.example.com", "time2.example.com"},
			expectedBootTimeout: 30 * time.Second,
		},
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectedNil: true,
		},
		{
			name: "new style only",

			cfg: func(*testing.T) config.Config {
				tsc := network.NewTimeSyncConfigV1Alpha1()
				tsc.TimeBootTimeout = 10 * time.Second
				tsc.TimeNTP = &network.NTPConfig{
					Servers: []string{"time1.example.com", "time2.example.com"},
				}

				c, err := container.New(
					tsc,
				)
				require.NoError(t, err)

				return c
			},

			expectedDisabled:    false,
			expectedTimeservers: []string{"time1.example.com", "time2.example.com"},
			expectedBootTimeout: 10 * time.Second,
		},
		{
			name: "mixed",

			cfg: func(*testing.T) config.Config {
				tsc := network.NewTimeSyncConfigV1Alpha1()
				tsc.TimeBootTimeout = 10 * time.Second
				tsc.TimeNTP = &network.NTPConfig{
					Servers: []string{"time1.example.com", "time2.example.com"},
				}

				c, err := container.New(
					tsc,
					&v1alpha1.Config{
						MachineConfig: &v1alpha1.MachineConfig{
							MachineNetwork: &v1alpha1.NetworkConfig{},
						},
					},
				)
				require.NoError(t, err)

				return c
			},

			expectedDisabled:    false,
			expectedTimeservers: []string{"time1.example.com", "time2.example.com"},
			expectedBootTimeout: 10 * time.Second,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			timesyncConfig := cfg.NetworkTimeSyncConfig()

			if test.expectedNil {
				require.Nil(t, timesyncConfig)

				return
			}

			require.NotNil(t, timesyncConfig)

			assert.Equal(t, test.expectedTimeservers, timesyncConfig.Servers())
			assert.Equal(t, test.expectedDisabled, timesyncConfig.Disabled())
			assert.Equal(t, test.expectedBootTimeout, timesyncConfig.BootTimeout())
		})
	}
}

func TestKubeSpanBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectedKubeSpanExists  bool
		expectedKubeSpanEnabled bool
	}{
		{
			name: "v1alpha1 only",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineNetwork: &v1alpha1.NetworkConfig{
							NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{
								KubeSpanEnabled: new(true),
							},
						},
					},
				})
			},

			expectedKubeSpanExists:  true,
			expectedKubeSpanEnabled: true,
		},
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectedKubeSpanExists:  false,
			expectedKubeSpanEnabled: false,
		},
		{
			name: "new style only",

			cfg: func(*testing.T) config.Config {
				kc := network.NewKubeSpanV1Alpha1()
				kc.ConfigEnabled = new(true)

				c, err := container.New(
					kc,
				)
				require.NoError(t, err)

				return c
			},

			expectedKubeSpanExists:  true,
			expectedKubeSpanEnabled: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			kubespanConfig := cfg.NetworkKubeSpanConfig()

			if !test.expectedKubeSpanExists {
				require.Nil(t, kubespanConfig)

				return
			}

			require.NotNil(t, kubespanConfig)

			assert.Equal(t, test.expectedKubeSpanEnabled, kubespanConfig.Enabled())
		})
	}
}

func TestHostDNSConfigBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectedHostDNSConfigExists bool
		expectedHostDNSEnabled      bool
	}{
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectedHostDNSConfigExists: false,
		},
		{
			name: "v1alpha1 hostDNS enabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy features
								HostDNSConfigEnabled: new(true),
							},
						},
					},
				})
			},

			expectedHostDNSConfigExists: true,
			expectedHostDNSEnabled:      true,
		},
		{
			name: "resolver config hostDNS enabled",

			cfg: func(*testing.T) config.Config {
				resolverCfg := network.NewResolverConfigV1Alpha1()
				resolverCfg.ResolverHostDNS = network.HostDNSConfig{
					HostDNSEnabled:              new(true),
					HostDNSForwardKubeDNSToHost: new(true),
					HostDNSResolveMemberNames:   new(true),
				}

				c, err := container.New(
					resolverCfg,
				)
				require.NoError(t, err)

				return c
			},

			expectedHostDNSConfigExists: true,
			expectedHostDNSEnabled:      true,
		},
		{
			name: "v1alpha1 empty and resolver config hostDNS enabled",

			cfg: func(*testing.T) config.Config {
				v1alpha1Cfg := &v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				}

				resolverCfg := network.NewResolverConfigV1Alpha1()
				resolverCfg.ResolverHostDNS = network.HostDNSConfig{
					HostDNSEnabled:              new(true),
					HostDNSForwardKubeDNSToHost: new(true),
					HostDNSResolveMemberNames:   new(true),
				}

				c, err := container.New(
					v1alpha1Cfg,
					resolverCfg,
				)
				require.NoError(t, err)

				return c
			},

			expectedHostDNSConfigExists: true,
			expectedHostDNSEnabled:      true,
		},
		{
			name: "v1alpha1 hostDNS enabled and resolver empty",

			cfg: func(*testing.T) config.Config {
				v1alpha1Cfg := &v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy features
								HostDNSConfigEnabled: new(true),
							},
						},
					},
				}

				resolverCfg := network.NewResolverConfigV1Alpha1()

				c, err := container.New(
					v1alpha1Cfg,
					resolverCfg,
				)
				require.NoError(t, err)

				return c
			},

			expectedHostDNSConfigExists: true,
			expectedHostDNSEnabled:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			hostDNSConfig := cfg.NetworkHostDNSConfig()

			if !test.expectedHostDNSConfigExists {
				require.Nil(t, hostDNSConfig)

				return
			}

			require.NotNil(t, hostDNSConfig)

			assert.Equal(t, test.expectedHostDNSEnabled, hostDNSConfig.HostDNSEnabled())
		})
	}
}

func TestImageCacheConfigBridging(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(*testing.T) config.Config

		expectedImageCacheConfigExists bool
		expectedImageCacheEnabled      bool
	}{
		{
			name: "v1alpha1 empty",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				})
			},

			expectedImageCacheConfigExists: false,
		},
		{
			name: "v1alpha1 image cache enabled",

			cfg: func(*testing.T) config.Config {
				return container.NewV1Alpha1(&v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{
						MachineFeatures: &v1alpha1.FeaturesConfig{
							ImageCacheSupport: &v1alpha1.ImageCacheConfig{ //nolint:staticcheck // testing legacy features
								CacheLocalEnabled: new(true),
							},
						},
					},
				})
			},

			expectedImageCacheConfigExists: true,
			expectedImageCacheEnabled:      true,
		},
		{
			name: "multi-doc image cache config enabled",

			cfg: func(*testing.T) config.Config {
				cacheCfg := cri.NewImageCacheConfigV1Alpha1()
				cacheCfg.LocalConfig.ConfigEnabled = new(true)

				c, err := container.New(
					cacheCfg,
				)
				require.NoError(t, err)

				return c
			},

			expectedImageCacheConfigExists: true,
			expectedImageCacheEnabled:      true,
		},
		{
			name: "v1alpha1 empty and image cache config enabled",

			cfg: func(*testing.T) config.Config {
				v1alpha1Cfg := &v1alpha1.Config{
					MachineConfig: &v1alpha1.MachineConfig{},
				}

				cacheCfg := cri.NewImageCacheConfigV1Alpha1()
				cacheCfg.LocalConfig.ConfigEnabled = new(true)

				c, err := container.New(
					v1alpha1Cfg,
					cacheCfg,
				)
				require.NoError(t, err)

				return c
			},

			expectedImageCacheConfigExists: true,
			expectedImageCacheEnabled:      true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			imageCacheConfig := cfg.ImageCacheConfig()

			if !test.expectedImageCacheConfigExists {
				require.Nil(t, imageCacheConfig)

				return
			}

			require.NotNil(t, imageCacheConfig)

			assert.Equal(t, test.expectedImageCacheEnabled, imageCacheConfig.LocalEnabled())
		})
	}
}
