// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed testdata/resolverconfig.yaml
var expectedResolverConfigDocument []byte

//go:embed testdata/resolverconfig_with_hostdns.yaml
var expectedResolverConfigDocumentWithHostDNS []byte

//go:embed testdata/resolverconfig_empty_search_domains.yaml
var expectedResolverConfigDocumentEmptySearchDomains []byte

func TestResolverConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewResolverConfigV1Alpha1()
	cfg.ResolverNameservers = []network.NameserverConfig{
		{
			Address: meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")},
		},
		{
			Address:       meta.Addr{Addr: netip.MustParseAddr("2001:4860:4860::8888")},
			Protocol:      nethelpers.DNSProtocolDNSOverTLS,
			TLSServerName: "dns.google",
		},
	}
	cfg.ResolverSearchDomains = network.SearchDomainsConfig{
		SearchDomains:        network.SearchDomainList{"example.org", "example.com"},
		SearchDisableDefault: new(false),
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedResolverConfigDocument, marshaled)
}

func TestResolverConfigMarshalStabilityWithHostDNS(t *testing.T) {
	t.Parallel()

	cfg := network.NewResolverConfigV1Alpha1()
	cfg.ResolverNameservers = []network.NameserverConfig{
		{
			Address: meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")},
		},
	}
	cfg.ResolverHostDNS = network.HostDNSConfig{
		HostDNSEnabled:              new(true),
		HostDNSForwardKubeDNSToHost: new(true),
		HostDNSResolveMemberNames:   new(false),
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedResolverConfigDocumentWithHostDNS, marshaled)
}

// TestResolverConfigMarshalStabilityEmptySearchDomains asserts that an explicitly empty
// list of search domains survives encoding.
//
// An empty list clears search domains obtained from DHCP or platform, so dropping it on
// encoding (as `omitempty` would) makes it impossible to persist.
func TestResolverConfigMarshalStabilityEmptySearchDomains(t *testing.T) {
	t.Parallel()

	cfg := network.NewResolverConfigV1Alpha1()
	cfg.ResolverSearchDomains = network.SearchDomainsConfig{
		SearchDomains:        network.SearchDomainList{},
		SearchDisableDefault: new(true),
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedResolverConfigDocumentEmptySearchDomains, marshaled)

	// the encoder takes a different code path when comments are enabled, so cover it as well
	marshaledWithComments, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsAll)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaledWithComments))

	assert.Contains(t, string(marshaledWithComments), "domains: []")
}

// TestResolverConfigMarshalUnsetSearchDomains asserts that an unset list of search domains
// is not encoded, as that would turn "inherit" into "clear" on the next write.
func TestResolverConfigMarshalUnsetSearchDomains(t *testing.T) {
	t.Parallel()

	cfg := network.NewResolverConfigV1Alpha1()
	cfg.ResolverSearchDomains = network.SearchDomainsConfig{
		SearchDisableDefault: new(true),
	}

	for _, comments := range []encoder.CommentsFlags{encoder.CommentsDisabled, encoder.CommentsAll} {
		marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(comments)).Encode()
		require.NoError(t, err)

		assert.NotContains(t, string(marshaled), "domains:")
	}
}

func TestResolverConfigUnmarshalEmptySearchDomains(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedResolverConfigDocumentEmptySearchDomains)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.ResolverConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.ResolverKind,
		},
		ResolverSearchDomains: network.SearchDomainsConfig{
			SearchDomains:        network.SearchDomainList{},
			SearchDisableDefault: new(true),
		},
	}, docs[0])

	searchDomains := provider.NetworkResolverConfig().SearchDomains()
	assert.True(t, searchDomains.IsPresent())
	assert.Empty(t, searchDomains.ValueOrZero())
}

// TestResolverConfigMergeSearchDomains asserts that a strategic merge patch replaces the
// list of search domains instead of appending to it, so that `domains: []` clears it.
func TestResolverConfigMergeSearchDomains(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		left     network.SearchDomainList
		right    network.SearchDomainList
		expected network.SearchDomainList
	}{
		{
			name:     "clear",
			left:     network.SearchDomainList{"example.org"},
			right:    network.SearchDomainList{},
			expected: network.SearchDomainList{},
		},
		{
			name:     "replace",
			left:     network.SearchDomainList{"example.org"},
			right:    network.SearchDomainList{"example.com"},
			expected: network.SearchDomainList{"example.com"},
		},
		{
			name:     "unset keeps left",
			left:     network.SearchDomainList{"example.org"},
			right:    nil,
			expected: network.SearchDomainList{"example.org"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			left := network.NewResolverConfigV1Alpha1()
			left.ResolverSearchDomains.SearchDomains = test.left

			right := network.NewResolverConfigV1Alpha1()
			right.ResolverSearchDomains.SearchDomains = test.right

			require.NoError(t, merge.Merge(left, right))

			assert.Equal(t, test.expected, left.ResolverSearchDomains.SearchDomains)
		})
	}
}

func TestResolverConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedResolverConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.ResolverConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.ResolverKind,
		},
		ResolverNameservers: []network.NameserverConfig{
			{
				Address: meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")},
			},
			{
				Address:       meta.Addr{Addr: netip.MustParseAddr("2001:4860:4860::8888")},
				Protocol:      nethelpers.DNSProtocolDNSOverTLS,
				TLSServerName: "dns.google",
			},
		},
		ResolverSearchDomains: network.SearchDomainsConfig{
			SearchDomains:        network.SearchDomainList{"example.org", "example.com"},
			SearchDisableDefault: new(false),
		},
	}, docs[0])
}

func TestResolverV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config
		cfg         func() *network.ResolverConfigV1Alpha1

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
			cfg:         network.NewResolverConfigV1Alpha1,
		},
		{
			name: "v1alpha1 nameservers set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NameServers: []string{"1.1.1.1"},
					},
				},
			},
			cfg: network.NewResolverConfigV1Alpha1,

			expectedError: ".machine.network.nameservers is already set in v1alpha1 config",
		},
		{
			name: "v1alpha1 search domains set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						Searches: []string{"cluster.org"},
					},
				},
			},
			cfg: network.NewResolverConfigV1Alpha1,

			expectedError: ".machine.network.searchDomains is already set in v1alpha1 config",
		},
		{
			name: "v1alpha1 disable search domains set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkDisableSearchDomain: new(true),
					},
				},
			},
			cfg: network.NewResolverConfigV1Alpha1,

			expectedError: ".machine.network.disableSearchDomain is already set in v1alpha1 config",
		},
		{
			name: "v1alpha1 hostDNS and resolver hostDNS set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy features
							HostDNSConfigEnabled:        new(true),
							HostDNSForwardKubeDNSToHost: new(true),
						},
					},
				},
			},
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverHostDNS = network.HostDNSConfig{
					HostDNSEnabled:              new(true),
					HostDNSForwardKubeDNSToHost: new(true),
				}

				return cfg
			},

			expectedError: ".machine.features.hostDNS is already set in v1alpha1 config",
		},
		{
			name: "v1alpha1 hostDNS and no resolver hostDNS set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy features
							HostDNSConfigEnabled:        new(true),
							HostDNSForwardKubeDNSToHost: new(true),
						},
					},
				},
			},
			cfg: network.NewResolverConfigV1Alpha1,
		},
		{
			name:        "v1alpha1 no hostDNS and resolver hostDNS set",
			v1alpha1Cfg: &v1alpha1.Config{},
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverHostDNS = network.HostDNSConfig{
					HostDNSEnabled:              new(true),
					HostDNSForwardKubeDNSToHost: new(true),
				}

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.cfg().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResolverV1Alpha1Validate(t *testing.T) {
	t.Parallel()

	const dotOnlyWarning = "all configured nameservers use encrypted DNS (DoT or DoH): validating certificates requires a correct system clock, " +
		"so boot may stall when NTP servers are configured by hostname; consider keeping at least one plain-DNS fallback " +
		"or configuring NTP servers by IP address"

	for _, test := range []struct {
		name string
		cfg  func() *network.ResolverConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  network.NewResolverConfigV1Alpha1,
		},
		{
			name: "forwardKubeDNSToHost true but HostDNSEnabled false",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverHostDNS = network.HostDNSConfig{
					HostDNSEnabled:              new(false),
					HostDNSForwardKubeDNSToHost: new(true),
				}

				return cfg
			},

			expectedError: "hostDNS.forwardKubeDNSToHost cannot be enabled when hostDNS.enabled is false",
		},
		{
			name: "resolveMemberNames true but HostDNSEnabled false",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverHostDNS = network.HostDNSConfig{
					HostDNSEnabled:            new(false),
					HostDNSResolveMemberNames: new(true),
				}

				return cfg
			},

			expectedError: "hostDNS.resolveMemberNames cannot be enabled when hostDNS.enabled is false",
		},
		{
			name: "hostDNS config valid",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverHostDNS = network.HostDNSConfig{
					HostDNSEnabled:              new(true),
					HostDNSForwardKubeDNSToHost: new(true),
					HostDNSResolveMemberNames:   new(false),
				}

				return cfg
			},
		},
		{
			name: "DoT mixed with plain DNS, no warning",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						Address:       meta.Addr{Addr: netip.MustParseAddr("9.9.9.9")},
						Protocol:      nethelpers.DNSProtocolDNSOverTLS,
						TLSServerName: "dns.quad9.net",
					},
					{
						Address: meta.Addr{Addr: netip.MustParseAddr("8.8.8.8")},
					},
				}

				return cfg
			},
		},
		{
			name: "DoT only, warns about clock dependency",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						Address:       meta.Addr{Addr: netip.MustParseAddr("9.9.9.9")},
						Protocol:      nethelpers.DNSProtocolDNSOverTLS,
						TLSServerName: "dns.quad9.net",
					},
					{
						Address:       meta.Addr{Addr: netip.MustParseAddr("1.1.1.1")},
						Protocol:      nethelpers.DNSProtocolDNSOverTLS,
						TLSServerName: "cloudflare-dns.com",
					},
				}

				return cfg
			},
			expectedWarnings: []string{dotOnlyWarning},
		},
		{
			name: "tlsServerName without an address",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						TLSServerName: "dns.quad9.net",
					},
				}

				return cfg
			},
			expectedError: "tlsServerName must be empty when protocol is Do53: entry 0\nnameserver address must be a valid IP: entry 0",
		},
		{
			name: "DoT without tlsServerName",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						Address:  meta.Addr{Addr: netip.MustParseAddr("9.9.9.9")},
						Protocol: nethelpers.DNSProtocolDNSOverTLS,
					},
				}

				return cfg
			},
			expectedError:    "tlsServerName must be set when protocol is DoT: entry 0",
			expectedWarnings: []string{dotOnlyWarning},
		},
		{
			name: "Do53 with tlsServerName set",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						Address:       meta.Addr{Addr: netip.MustParseAddr("8.8.8.8")},
						Protocol:      nethelpers.DNSProtocolDefault,
						TLSServerName: "dns.google",
					},
				}

				return cfg
			},
			expectedError: "tlsServerName must be empty when protocol is Do53: entry 0",
		},
		{
			name: "DoH with tlsServerName set",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						Address:       meta.Addr{Addr: netip.MustParseAddr("1.1.1.1")},
						Protocol:      nethelpers.DNSProtocolDNSOverHTTP,
						TLSServerName: "cloudflare-dns.com",
					},
					{
						Address: meta.Addr{Addr: netip.MustParseAddr("8.8.8.8")},
					},
				}

				return cfg
			},
		},
		{
			name: "DoH without tlsServerName",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						Address:  meta.Addr{Addr: netip.MustParseAddr("1.1.1.1")},
						Protocol: nethelpers.DNSProtocolDNSOverHTTP,
					},
				}

				return cfg
			},
			expectedError:    "tlsServerName must be set when protocol is DoH: entry 0",
			expectedWarnings: []string{dotOnlyWarning},
		},
		{
			name: "Mixed DoT and DoH only, warns about clock dependency",
			cfg: func() *network.ResolverConfigV1Alpha1 {
				cfg := network.NewResolverConfigV1Alpha1()
				cfg.ResolverNameservers = []network.NameserverConfig{
					{
						Address:       meta.Addr{Addr: netip.MustParseAddr("9.9.9.9")},
						Protocol:      nethelpers.DNSProtocolDNSOverTLS,
						TLSServerName: "dns.quad9.net",
					},
					{
						Address:       meta.Addr{Addr: netip.MustParseAddr("1.1.1.1")},
						Protocol:      nethelpers.DNSProtocolDNSOverHTTP,
						TLSServerName: "cloudflare-dns.com",
					},
				}

				return cfg
			},
			expectedWarnings: []string{dotOnlyWarning},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})
			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
