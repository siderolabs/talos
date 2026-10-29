// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type redactTestCase[T any] struct {
	name string
	// spec builds the input spec; it is called twice, so that the test can verify that redacting
	// a copy doesn't reach back into the original (e.g. via a shared slice).
	spec     func() T
	expected T
}

// runRedactTests exercises the copy-then-redact-in-place sequence the callers of the interface use.
func runRedactTests[T any, PT redactable.SpecPtr[T]](t *testing.T, md resource.Metadata, testCases []redactTestCase[T],
) {
	t.Helper()

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := test.spec()
			hasSecrets := PT(&spec).HasSecrets(md)

			redacted := PT(&spec).DeepCopy()
			PT(&redacted).RedactSecrets(md)

			assert.Equal(t, test.expected, redacted)
			assert.Equal(t, test.spec(), spec, "redacting a copy should not mutate the spec it was copied from")

			// HasSecrets decides whether the copy is made at all, so it has to report true
			// whenever redaction changes anything.
			if !assert.ObjectsAreEqual(spec, redacted) {
				assert.True(t, hasSecrets, "HasSecrets should report true for a spec which redaction changes")
			}
		})
	}
}

func TestLinkSpecRedactSecrets(t *testing.T) {
	t.Parallel()

	md := resource.NewMetadata(network.NamespaceName, network.LinkSpecType, "eth0", resource.VersionUndefined)

	runRedactTests(t, md, []redactTestCase[network.LinkSpecSpec]{
		{
			name: "no wireguard",
			spec: func() network.LinkSpecSpec {
				return network.LinkSpecSpec{
					Name:        "eth0",
					Up:          true,
					MTU:         1500,
					Type:        nethelpers.LinkEther,
					ConfigLayer: network.ConfigMachineConfiguration,
				}
			},
			expected: network.LinkSpecSpec{
				Name:        "eth0",
				Up:          true,
				MTU:         1500,
				Type:        nethelpers.LinkEther,
				ConfigLayer: network.ConfigMachineConfiguration,
			},
		},
		{
			name: "wireguard with private and preshared keys",
			spec: func() network.LinkSpecSpec {
				return network.LinkSpecSpec{
					Name:    "kubespan",
					Logical: true,
					Kind:    "wireguard",
					Wireguard: network.WireguardSpec{
						PrivateKey:   "GMFVMbEwHfSKgxRT6JQjGnUwAg7pTh1fbCsuJnrCiXo=",
						PublicKey:    "RcRXvGgWQFqOTfSlwOKY7CGwWvVAqBGYsjMEIcJ5vXQ=",
						ListenPort:   51820,
						FirewallMark: 42,
						Peers: []network.WireguardPeer{
							{
								PublicKey:                   "PLPNmk6Yr9Zh6gAv+RTIhUEZmg8UNK6qGa6y0T2ZOWM=",
								PresharedKey:                "1lSlUlLIhRnvJdD9wLL/rMuHUCtBHnQTA/1kDmVYEQ4=",
								Endpoint:                    "10.0.0.1:51820",
								PersistentKeepaliveInterval: 25 * time.Second,
								AllowedIPs:                  []netip.Prefix{netip.MustParsePrefix("10.244.0.0/24")},
							},
							{
								PublicKey:  "ZJmXFOrqYAOEAyPKMSTIvzAUYIQPzOOLjF0jrKmxHFI=",
								Endpoint:   "10.0.0.2:51820",
								AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.244.1.0/24")},
							},
						},
					},
					ConfigLayer: network.ConfigMachineConfiguration,
				}
			},
			expected: network.LinkSpecSpec{
				Name:    "kubespan",
				Logical: true,
				Kind:    "wireguard",
				Wireguard: network.WireguardSpec{
					PrivateKey:   constants.Redacted,
					PublicKey:    "RcRXvGgWQFqOTfSlwOKY7CGwWvVAqBGYsjMEIcJ5vXQ=",
					ListenPort:   51820,
					FirewallMark: 42,
					Peers: []network.WireguardPeer{
						{
							PublicKey:                   "PLPNmk6Yr9Zh6gAv+RTIhUEZmg8UNK6qGa6y0T2ZOWM=",
							PresharedKey:                constants.Redacted,
							Endpoint:                    "10.0.0.1:51820",
							PersistentKeepaliveInterval: 25 * time.Second,
							AllowedIPs:                  []netip.Prefix{netip.MustParsePrefix("10.244.0.0/24")},
						},
						{
							PublicKey:  "ZJmXFOrqYAOEAyPKMSTIvzAUYIQPzOOLjF0jrKmxHFI=",
							Endpoint:   "10.0.0.2:51820",
							AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.244.1.0/24")},
						},
					},
				},
				ConfigLayer: network.ConfigMachineConfiguration,
			},
		},
		{
			name: "wireguard without secrets set",
			spec: func() network.LinkSpecSpec {
				return network.LinkSpecSpec{
					Name: "wg0",
					Kind: "wireguard",
					Wireguard: network.WireguardSpec{
						PublicKey: "RcRXvGgWQFqOTfSlwOKY7CGwWvVAqBGYsjMEIcJ5vXQ=",
						Peers: []network.WireguardPeer{
							{
								PublicKey: "PLPNmk6Yr9Zh6gAv+RTIhUEZmg8UNK6qGa6y0T2ZOWM=",
								Endpoint:  "10.0.0.1:51820",
							},
						},
					},
				}
			},
			expected: network.LinkSpecSpec{
				Name: "wg0",
				Kind: "wireguard",
				Wireguard: network.WireguardSpec{
					PublicKey: "RcRXvGgWQFqOTfSlwOKY7CGwWvVAqBGYsjMEIcJ5vXQ=",
					Peers: []network.WireguardPeer{
						{
							PublicKey: "PLPNmk6Yr9Zh6gAv+RTIhUEZmg8UNK6qGa6y0T2ZOWM=",
							Endpoint:  "10.0.0.1:51820",
						},
					},
				},
			},
		},
	})
}

func TestOperatorSpecRedactSecrets(t *testing.T) {
	t.Parallel()

	md := resource.NewMetadata(network.NamespaceName, network.OperatorSpecType, "vip/eth0", resource.VersionUndefined)

	runRedactTests(t, md, []redactTestCase[network.OperatorSpecSpec]{
		{
			name: "dhcp4",
			spec: func() network.OperatorSpecSpec {
				return network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP4,
					LinkName:  "eth0",
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: 1024,
					},
					ConfigLayer: network.ConfigMachineConfiguration,
				}
			},
			expected: network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  "eth0",
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: 1024,
				},
				ConfigLayer: network.ConfigMachineConfiguration,
			},
		},
		{
			name: "vip with equinix metal api token",
			spec: func() network.OperatorSpecSpec {
				return network.OperatorSpecSpec{
					Operator: network.OperatorVIP,
					LinkName: "eth0",
					VIP: network.VIPOperatorSpec{
						IP:            netip.MustParseAddr("10.0.0.1"),
						GratuitousARP: true,
						EquinixMetal: network.VIPEquinixMetalSpec{
							ProjectID: "project-id",
							DeviceID:  "device-id",
							APIToken:  "super-secret-token",
						},
					},
					ConfigLayer: network.ConfigMachineConfiguration,
				}
			},
			expected: network.OperatorSpecSpec{
				Operator: network.OperatorVIP,
				LinkName: "eth0",
				VIP: network.VIPOperatorSpec{
					IP:            netip.MustParseAddr("10.0.0.1"),
					GratuitousARP: true,
					EquinixMetal: network.VIPEquinixMetalSpec{
						ProjectID: "project-id",
						DeviceID:  "device-id",
						APIToken:  constants.Redacted,
					},
				},
				ConfigLayer: network.ConfigMachineConfiguration,
			},
		},
		{
			name: "vip with hcloud api token",
			spec: func() network.OperatorSpecSpec {
				return network.OperatorSpecSpec{
					Operator: network.OperatorVIP,
					LinkName: "eth0",
					VIP: network.VIPOperatorSpec{
						IP: netip.MustParseAddr("10.0.0.1"),
						HCloud: network.VIPHCloudSpec{
							DeviceID:  42,
							NetworkID: 24,
							APIToken:  "super-secret-token",
						},
					},
				}
			},
			expected: network.OperatorSpecSpec{
				Operator: network.OperatorVIP,
				LinkName: "eth0",
				VIP: network.VIPOperatorSpec{
					IP: netip.MustParseAddr("10.0.0.1"),
					HCloud: network.VIPHCloudSpec{
						DeviceID:  42,
						NetworkID: 24,
						APIToken:  constants.Redacted,
					},
				},
			},
		},
		{
			name: "vip without api tokens",
			spec: func() network.OperatorSpecSpec {
				return network.OperatorSpecSpec{
					Operator: network.OperatorVIP,
					LinkName: "eth0",
					VIP: network.VIPOperatorSpec{
						IP:            netip.MustParseAddr("10.0.0.1"),
						GratuitousARP: true,
					},
				}
			},
			expected: network.OperatorSpecSpec{
				Operator: network.OperatorVIP,
				LinkName: "eth0",
				VIP: network.VIPOperatorSpec{
					IP:            netip.MustParseAddr("10.0.0.1"),
					GratuitousARP: true,
				},
			},
		},
	})
}

func TestProbeSpecRedactSecrets(t *testing.T) {
	t.Parallel()

	md := resource.NewMetadata(network.NamespaceName, network.ProbeSpecType, "http:https://example.com/health", resource.VersionUndefined)

	runRedactTests(t, md, []redactTestCase[network.ProbeSpecSpec]{
		{
			name: "tcp",
			spec: func() network.ProbeSpecSpec {
				return network.ProbeSpecSpec{
					Interval:         time.Second,
					FailureThreshold: 3,
					TCP: network.TCPProbeSpec{
						Endpoint: "example.com:80",
						Timeout:  time.Second,
					},
					ConfigLayer: network.ConfigMachineConfiguration,
				}
			},
			expected: network.ProbeSpecSpec{
				Interval:         time.Second,
				FailureThreshold: 3,
				TCP: network.TCPProbeSpec{
					Endpoint: "example.com:80",
					Timeout:  time.Second,
				},
				ConfigLayer: network.ConfigMachineConfiguration,
			},
		},
		{
			name: "http without credentials",
			spec: func() network.ProbeSpecSpec {
				return network.ProbeSpecSpec{
					Interval: time.Second,
					HTTP: network.HTTPProbeSpec{
						URL:     mustParseURL("https://example.com/health"),
						Timeout: time.Second,
					},
				}
			},
			expected: network.ProbeSpecSpec{
				Interval: time.Second,
				HTTP: network.HTTPProbeSpec{
					URL:     mustParseURL("https://example.com/health"),
					Timeout: time.Second,
				},
			},
		},
		{
			name: "http with password",
			spec: func() network.ProbeSpecSpec {
				return network.ProbeSpecSpec{
					Interval: time.Second,
					HTTP: network.HTTPProbeSpec{
						URL:     mustParseURL("https://user:super-secret@example.com/health?foo=bar"),
						Timeout: time.Second,
					},
				}
			},
			expected: network.ProbeSpecSpec{
				Interval: time.Second,
				HTTP: network.HTTPProbeSpec{
					URL:     mustParseURL("https://user:" + constants.Redacted + "@example.com/health?foo=bar"),
					Timeout: time.Second,
				},
			},
		},
		{
			name: "http with username only",
			spec: func() network.ProbeSpecSpec {
				return network.ProbeSpecSpec{
					Interval: time.Second,
					HTTP: network.HTTPProbeSpec{
						URL:     mustParseURL("https://user@example.com/health"),
						Timeout: time.Second,
					},
				}
			},
			expected: network.ProbeSpecSpec{
				Interval: time.Second,
				HTTP: network.HTTPProbeSpec{
					URL:     mustParseURL("https://user:" + constants.Redacted + "@example.com/health"),
					Timeout: time.Second,
				},
			},
		},
	})
}

func TestBGPInstanceConfigRedactSecrets(t *testing.T) {
	t.Parallel()

	md := resource.NewMetadata(network.NamespaceName, network.BGPInstanceConfigType, "fabric", resource.VersionUndefined)

	runRedactTests(t, md, []redactTestCase[network.BGPInstanceConfigSpec]{
		{
			name: "no neighbors",
			spec: func() network.BGPInstanceConfigSpec {
				return network.BGPInstanceConfigSpec{
					LocalASN: 65001,
					RouterID: netip.MustParseAddr("10.0.0.1"),
				}
			},
			expected: network.BGPInstanceConfigSpec{
				LocalASN: 65001,
				RouterID: netip.MustParseAddr("10.0.0.1"),
			},
		},
		{
			name: "neighbors with and without auth password",
			spec: func() network.BGPInstanceConfigSpec {
				return network.BGPInstanceConfigSpec{
					LocalASN: 65001,
					RouterID: netip.MustParseAddr("10.0.0.1"),
					Neighbors: []network.BGPNeighborConfigSpec{
						{
							Address:      netip.MustParseAddr("192.0.2.1"),
							PeerASN:      65000,
							AuthPassword: "s3cr3t",
						},
						{
							Link:    "eth1",
							PeerASN: 65002,
						},
					},
				}
			},
			expected: network.BGPInstanceConfigSpec{
				LocalASN: 65001,
				RouterID: netip.MustParseAddr("10.0.0.1"),
				Neighbors: []network.BGPNeighborConfigSpec{
					{
						Address:      netip.MustParseAddr("192.0.2.1"),
						PeerASN:      65000,
						AuthPassword: constants.Redacted,
					},
					{
						Link:    "eth1",
						PeerASN: 65002,
					},
				},
			},
		},
	})
}

// TestRedactSecretsNoSecrets covers the specs which carry no sensitive data: RedactSecrets
// is expected to pass them through unchanged.
func TestRedactSecretsNoSecrets(t *testing.T) {
	t.Parallel()

	addressSpec := network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.1/24"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	runRedactTests(t, resource.NewMetadata(network.NamespaceName, network.AddressSpecType, "eth0/10.0.0.1/24", resource.VersionUndefined),
		[]redactTestCase[network.AddressSpecSpec]{
			{
				name:     "address",
				spec:     func() network.AddressSpecSpec { return addressSpec },
				expected: addressSpec,
			},
		})

	hostnameSpec := network.HostnameSpecSpec{
		Hostname:    "talos-node",
		Domainname:  "example.com",
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	runRedactTests(t, resource.NewMetadata(network.NamespaceName, network.HostnameSpecType, "hostname", resource.VersionUndefined),
		[]redactTestCase[network.HostnameSpecSpec]{
			{
				name:     "hostname",
				spec:     func() network.HostnameSpecSpec { return hostnameSpec },
				expected: hostnameSpec,
			},
		})

	runRedactTests(t, resource.NewMetadata(network.NamespaceName, network.ResolverSpecType, "resolvers", resource.VersionUndefined),
		[]redactTestCase[network.ResolverSpecSpec]{
			{
				name: "resolver",
				spec: func() network.ResolverSpecSpec {
					return network.ResolverSpecSpec{
						DNSServers: []netip.Addr{netip.MustParseAddr("1.1.1.1")},
						NameServers: []network.NameServerSpec{
							{
								Addr:          netip.MustParseAddr("1.1.1.1"),
								Protocol:      nethelpers.DNSProtocolDNSOverTLS,
								TLSServerName: "cloudflare-dns.com",
							},
						},
						SearchDomains: []string{"example.com"},
						ConfigLayer:   network.ConfigMachineConfiguration,
					}
				},
				expected: network.ResolverSpecSpec{
					DNSServers: []netip.Addr{netip.MustParseAddr("1.1.1.1")},
					NameServers: []network.NameServerSpec{
						{
							Addr:          netip.MustParseAddr("1.1.1.1"),
							Protocol:      nethelpers.DNSProtocolDNSOverTLS,
							TLSServerName: "cloudflare-dns.com",
						},
					},
					SearchDomains: []string{"example.com"},
					ConfigLayer:   network.ConfigMachineConfiguration,
				},
			},
		})

	routeSpec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("10.0.0.0/8"),
		Gateway:     netip.MustParseAddr("10.0.0.1"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableMain,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Protocol:    nethelpers.ProtocolStatic,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	routeID := network.RouteID(routeSpec.Table, routeSpec.Family, routeSpec.Destination, routeSpec.Priority)

	runRedactTests(t, resource.NewMetadata(network.NamespaceName, network.RouteSpecType, routeID, resource.VersionUndefined),
		[]redactTestCase[network.RouteSpecSpec]{
			{
				name:     "route",
				spec:     func() network.RouteSpecSpec { return routeSpec },
				expected: routeSpec,
			},
		})

	routingRuleSpec := network.RoutingRuleSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Src:         netip.MustParsePrefix("10.0.0.1/32"),
		Table:       nethelpers.TableMain,
		Priority:    100,
		Action:      nethelpers.RoutingRuleActionUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	runRedactTests(t, resource.NewMetadata(network.NamespaceName, network.RoutingRuleSpecType, "rule", resource.VersionUndefined),
		[]redactTestCase[network.RoutingRuleSpecSpec]{
			{
				name:     "routing rule",
				spec:     func() network.RoutingRuleSpecSpec { return routingRuleSpec },
				expected: routingRuleSpec,
			},
		})

	runRedactTests(t, resource.NewMetadata(network.NamespaceName, network.TimeServerSpecType, "timeservers", resource.VersionUndefined),
		[]redactTestCase[network.TimeServerSpecSpec]{
			{
				name: "time server",
				spec: func() network.TimeServerSpecSpec {
					return network.TimeServerSpecSpec{
						NTPServers:  []string{"time.cloudflare.com"},
						UseNTS:      true,
						ConfigLayer: network.ConfigMachineConfiguration,
					}
				},
				expected: network.TimeServerSpecSpec{
					NTPServers:  []string{"time.cloudflare.com"},
					UseNTS:      true,
					ConfigLayer: network.ConfigMachineConfiguration,
				},
			},
		})
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	return u
}

// TestMetalNetworkPlatformConfigRedaction covers the redaction of the MetalNetworkPlatformConfig META
// key, whose value is a marshaled PlatformConfigSpec.
//
// The network package registers a redactor for that key, so that a config which carries no secret at
// all -- which is all the dashboard ever writes: static or DHCP links, addresses and routes -- is
// served untouched instead of being blanked out wholesale.
func TestMetalNetworkPlatformConfigRedaction(t *testing.T) {
	t.Parallel()

	marshal := func(spec *network.PlatformConfigSpec) string {
		out, err := yaml.Marshal(spec)
		require.NoError(t, err)

		return string(out)
	}

	plain := &network.PlatformConfigSpec{
		Addresses: []network.AddressSpecSpec{
			{
				Address:  netip.MustParsePrefix("192.168.1.10/24"),
				LinkName: "eth0",
				Family:   nethelpers.FamilyInet4,
				Scope:    nethelpers.ScopeGlobal,
				// the flags the dashboard sets on a static address; a zero value doesn't survive the
				// YAML round trip, see nethelpers.AddressFlagsString
				Flags: nethelpers.AddressFlags(nethelpers.AddressPermanent),
			},
		},
		Links: []network.LinkSpecSpec{
			{
				Name: "eth0",
				Up:   true,
			},
		},
		Hostnames: []network.HostnameSpecSpec{
			{
				Hostname: "node-1",
			},
		},
	}

	withWireguard := &network.PlatformConfigSpec{
		Addresses: plain.Addresses,
		Links: append(slices.Clone(plain.Links), network.LinkSpecSpec{
			Name:    "kubespan",
			Logical: true,
			Up:      true,
			Kind:    network.LinkKindWireguard,
			Wireguard: network.WireguardSpec{
				PrivateKey: "5bPRvlMKtOwjnjVBcvIrhEJiTA6hHDYRTG4bXAMWKYA=",
			},
		}),
		Hostnames: plain.Hostnames,
	}

	for _, test := range []struct {
		name  string
		value string
		// assert checks the redacted value.
		assert     func(t *testing.T, redacted string)
		hasSecrets bool
	}{
		{
			name:  "no secrets",
			value: marshal(plain),
			assert: func(t *testing.T, redacted string) {
				assert.Equal(t, marshal(plain), redacted, "a config with nothing to redact should be served byte for byte")
			},
		},
		{
			name:       "wireguard key",
			value:      marshal(withWireguard),
			hasSecrets: true,
			assert: func(t *testing.T, redacted string) {
				var spec network.PlatformConfigSpec

				require.NoError(t, yaml.Unmarshal([]byte(redacted), &spec))

				assert.Equal(t, constants.Redacted, spec.Links[1].Wireguard.PrivateKey)
				// the rest of the config survives, which is the point of redacting it rather than denying it
				assert.Equal(t, plain.Addresses, spec.Addresses)
				assert.Equal(t, plain.Hostnames, spec.Hostnames)
				assert.Equal(t, "kubespan", spec.Links[1].Name)
			},
		},
		{
			name:       "not a config",
			value:      "whatever was written into the key",
			hasSecrets: true,
			assert: func(t *testing.T, redacted string) {
				assert.Equal(t, constants.Redacted, redacted, "a value which can't be parsed should be assumed to hold secrets")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			key := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(metaconsts.MetalNetworkPlatformConfig))
			key.TypedSpec().Value = test.value

			md := *key.Metadata()

			assert.Equal(t, test.hasSecrets, key.TypedSpec().HasSecrets(md))

			key.TypedSpec().RedactSecrets(md)

			test.assert(t, key.TypedSpec().Value)
		})
	}
}
