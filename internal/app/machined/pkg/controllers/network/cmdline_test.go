// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"cmp"
	"fmt"
	"iter"
	"net"
	"net/netip"
	"slices"
	"testing"

	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	netconfig "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestCmdlineParse(t *testing.T) {
	// [NOTE]: this test is not safe to run in parallel, as defaultIfaceName might flip if some interface is created concurrently
	ifaces, _ := net.Interfaces() //nolint:errcheck // ignoring error here as ifaces will be empty

	slices.SortFunc(ifaces, func(a, b net.Interface) int { return cmp.Compare(a.Name, b.Name) })

	defaultIfaceName := ""

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		defaultIfaceName = iface.Name

		break
	}

	defaultBondSettings := network.CmdlineNetworking{
		NetworkLinkSpecs: []netconfig.LinkSpecSpec{
			{
				Name:        "bond1",
				Kind:        "bond",
				Type:        nethelpers.LinkEther,
				Logical:     true,
				Up:          true,
				ConfigLayer: netconfig.ConfigCmdline,
				BondMaster: netconfig.BondMasterSpec{
					Mode:            nethelpers.BondModeRoundrobin,
					ResendIGMP:      1,
					LPInterval:      1,
					PacketsPerSlave: 1,
					NumPeerNotif:    1,
					TLBDynamicLB:    1,
					UseCarrier:      true,
				},
			},
			{
				Name:        "eth0",
				Up:          true,
				Logical:     false,
				ConfigLayer: netconfig.ConfigCmdline,
				BondSlave: netconfig.BondSlave{
					MasterName: "bond1",
					SlaveIndex: 0,
				},
			},
			{
				Name:        "eth1",
				Up:          true,
				Logical:     false,
				ConfigLayer: netconfig.ConfigCmdline,
				BondSlave: netconfig.BondSlave{
					MasterName: "bond1",
					SlaveIndex: 1,
				},
			},
		},
	}

	for _, test := range []struct {
		name    string
		cmdline string

		expectedSettings network.CmdlineNetworking
		expectedError    string
	}{
		{
			name:    "zero",
			cmdline: "",
		},
		{
			name:    "static IP",
			cmdline: "ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1:::::",

			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("172.20.0.2/24"),
						Gateway:  netip.MustParseAddr("172.20.0.1"),
						LinkName: "eth1",
					},
				},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "no iface",
			cmdline: "ip=172.20.0.2::172.20.0.1",

			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("172.20.0.2/32"),
						Gateway:  netip.MustParseAddr("172.20.0.1"),
						LinkName: defaultIfaceName,
					},
				},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        defaultIfaceName,
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "no iface by mac address",
			cmdline: "ip=172.20.0.2::172.20.0.1:255.255.255.0::enx001122aabbcc",

			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("172.20.0.2/24"),
						Gateway:  netip.MustParseAddr("172.20.0.1"),
						LinkName: "enx001122aabbcc",
					},
				},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "enx001122aabbcc",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "complete",
			cmdline: "ip=172.20.0.2:172.21.0.1:172.20.0.1:255.255.255.0:master1:eth1::10.0.0.1:10.0.0.2:10.0.0.1",

			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("172.20.0.2/24"),
						Gateway:  netip.MustParseAddr("172.20.0.1"),
						LinkName: "eth1",
					},
				},
				Hostname:     "master1",
				DNSAddresses: []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")},
				NTPAddresses: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "another config",
			cmdline: "ip=10.105.155.21::10.105.155.30:255.255.255.240:pve1:eth0:off",

			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						LinkName: "eth0",
						Address:  netip.MustParsePrefix("10.105.155.21/28"),
						Gateway:  netip.MustParseAddr("10.105.155.30"),
					},
				},
				Hostname: "pve1",
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth0",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "ipv6",
			cmdline: "ip=[2001:db8::a]:[2001:db8::b]:[fe80::1]::master1:eth1::[2001:4860:4860::6464]:[2001:4860:4860::64]:[2001:4860:4806::]",
			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("2001:db8::a/128"),
						Gateway:  netip.MustParseAddr("fe80::1"),
						LinkName: "eth1",
					},
				},
				Hostname:     "master1",
				DNSAddresses: []netip.Addr{netip.MustParseAddr("2001:4860:4860::6464"), netip.MustParseAddr("2001:4860:4860::64")},
				NTPAddresses: []netip.Addr{netip.MustParseAddr("2001:4860:4806::")},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "ipv6-mask",
			cmdline: "ip=[2a03:1:2::12]::[2a03:1:2::11]:[ffff:ffff:ffff:ffff:ffff:ffff:ffff:fff8]:master:eth0:off:[2001:4860:4860::8888]:[2606:4700::1111]:[2606:4700:f1::1]",
			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("2a03:1:2::12/125"),
						Gateway:  netip.MustParseAddr("2a03:1:2::11"),
						LinkName: "eth0",
					},
				},
				Hostname:     "master",
				DNSAddresses: []netip.Addr{netip.MustParseAddr("2001:4860:4860::8888"), netip.MustParseAddr("2606:4700::1111")},
				NTPAddresses: []netip.Addr{netip.MustParseAddr("2606:4700:f1::1")},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth0",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "ipv6-mask-number",
			cmdline: "ip=[2a03:1:2::12]::[2a03:1:2::11]:125:master:eth0:off:[2001:4860:4860::8888]:[2606:4700::1111]:[2606:4700:f1::1]",
			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("2a03:1:2::12/125"),
						Gateway:  netip.MustParseAddr("2a03:1:2::11"),
						LinkName: "eth0",
					},
				},
				Hostname:     "master",
				DNSAddresses: []netip.Addr{netip.MustParseAddr("2001:4860:4860::8888"), netip.MustParseAddr("2606:4700::1111")},
				NTPAddresses: []netip.Addr{netip.MustParseAddr("2606:4700:f1::1")},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth0",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "unparseable IP",
			cmdline: "ip=xyz:",

			expectedError: "cmdline address parse failure: ParseAddr(\"xyz\"): unable to parse IP",
		},
		{
			name:    "hostname override",
			cmdline: "ip=::::master1:eth1 talos.hostname=master2",

			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						LinkName: "eth1",
					},
				},
				Hostname: "master2",
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
		{
			name:    "only hostname",
			cmdline: "talos.hostname=master2",

			expectedSettings: network.CmdlineNetworking{
				Hostname: "master2",
			},
		},
		{
			name:    "ignore interfaces",
			cmdline: "talos.network.interface.ignore=eth2 talos.network.interface.ignore=eth3 talos.network.interface.ignore=enxa",

			expectedSettings: network.CmdlineNetworking{
				IgnoreInterfaces: []string{"eth2", "eth3", "eth31"},
			},
		},
		{
			name:             "bond with no interfaces and no options set",
			cmdline:          "bond=bond1",
			expectedSettings: defaultBondSettings,
		},
		{
			name:             "bond with no interfaces and empty options set",
			cmdline:          "bond=bond1:::",
			expectedSettings: defaultBondSettings,
		},
		{
			name:    "bond with interfaces and no options set",
			cmdline: "bond=bond1:eth3,eth4",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					defaultBondSettings.NetworkLinkSpecs[0],
					{
						Name:        "eth3",
						Up:          true,
						Logical:     false,
						ConfigLayer: netconfig.ConfigCmdline,
						BondSlave: netconfig.BondSlave{
							MasterName: "bond1",
							SlaveIndex: 0,
						},
					},
					{
						Name:        "eth4",
						Up:          true,
						Logical:     false,
						ConfigLayer: netconfig.ConfigCmdline,
						BondSlave: netconfig.BondSlave{
							MasterName: "bond1",
							SlaveIndex: 1,
						},
					},
				},
			},
		},
		{
			name:    "bond with aliased interfaces",
			cmdline: "bond=bond1:enxa,enxb",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					defaultBondSettings.NetworkLinkSpecs[0],
					{
						Name:        "eth31",
						Up:          true,
						Logical:     false,
						ConfigLayer: netconfig.ConfigCmdline,
						BondSlave: netconfig.BondSlave{
							MasterName: "bond1",
							SlaveIndex: 0,
						},
					},
					{
						Name:        "eth32",
						Up:          true,
						Logical:     false,
						ConfigLayer: netconfig.ConfigCmdline,
						BondSlave: netconfig.BondSlave{
							MasterName: "bond1",
							SlaveIndex: 1,
						},
					},
				},
			},
		},
		{
			name:    "bond with interfaces, options and mtu set",
			cmdline: "bond=bond1:eth3,eth4:mode=802.3ad,xmit_hash_policy=layer2+3:1450",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "bond1",
						Kind:        "bond",
						Type:        nethelpers.LinkEther,
						Logical:     true,
						Up:          true,
						MTU:         1450,
						ConfigLayer: netconfig.ConfigCmdline,
						BondMaster: netconfig.BondMasterSpec{
							Mode:            nethelpers.BondMode8023AD,
							HashPolicy:      nethelpers.BondXmitPolicyLayer23,
							ADActorSysPrio:  65535,
							ResendIGMP:      1,
							LPInterval:      1,
							PacketsPerSlave: 1,
							NumPeerNotif:    1,
							TLBDynamicLB:    1,
							UseCarrier:      true,
						},
					},
					{
						Name:        "eth3",
						Up:          true,
						Logical:     false,
						ConfigLayer: netconfig.ConfigCmdline,
						BondSlave: netconfig.BondSlave{
							MasterName: "bond1",
							SlaveIndex: 0,
						},
					},
					{
						Name:        "eth4",
						Up:          true,
						Logical:     false,
						ConfigLayer: netconfig.ConfigCmdline,
						BondSlave: netconfig.BondSlave{
							MasterName: "bond1",
							SlaveIndex: 1,
						},
					},
				},
			},
		},
		{
			name:    "unparseable bond options",
			cmdline: "bond=bond0:eth1,eth2:mod=balance-rr",

			expectedError: "unknown bond option: mod",
		},
		{
			name:    "vlan configuration",
			cmdline: "vlan=eth1.169:eth1 ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1.169:::::",
			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("172.20.0.2/24"),
						Gateway:  netip.MustParseAddr("172.20.0.1"),
						LinkName: "eth1.169",
					},
				},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1.169",
						Logical:     true,
						Up:          true,
						Kind:        netconfig.LinkKindVLAN,
						Type:        nethelpers.LinkEther,
						ParentName:  "eth1",
						ConfigLayer: netconfig.ConfigCmdline,
						VLAN: netconfig.VLANSpec{
							VID:      169,
							Protocol: nethelpers.VLANProtocol8021Q,
						},
					},
				},
			},
		},
		{
			name:    "vlan configuration without ip configuration",
			cmdline: "vlan=eth1.5:eth1",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1.5",
						Logical:     true,
						Up:          true,
						Kind:        netconfig.LinkKindVLAN,
						Type:        nethelpers.LinkEther,
						ParentName:  "eth1",
						ConfigLayer: netconfig.ConfigCmdline,
						VLAN: netconfig.VLANSpec{
							VID:      5,
							Protocol: nethelpers.VLANProtocol8021Q,
						},
					},
				},
			},
		},
		{
			name:    "vlan configuration with alternative link name vlan0008",
			cmdline: "vlan=vlan0008:eth1",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1.8",
						Logical:     true,
						Up:          true,
						Kind:        netconfig.LinkKindVLAN,
						Type:        nethelpers.LinkEther,
						ParentName:  "eth1",
						ConfigLayer: netconfig.ConfigCmdline,
						VLAN: netconfig.VLANSpec{
							VID:      8,
							Protocol: nethelpers.VLANProtocol8021Q,
						},
					},
				},
			},
		},
		{
			name:    "vlan configuration with alternative link name vlan1",
			cmdline: "vlan=vlan4095:eth1",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1.4095",
						Logical:     true,
						Up:          true,
						Kind:        netconfig.LinkKindVLAN,
						Type:        nethelpers.LinkEther,
						ParentName:  "eth1",
						ConfigLayer: netconfig.ConfigCmdline,
						VLAN: netconfig.VLANSpec{
							VID:      4095,
							Protocol: nethelpers.VLANProtocol8021Q,
						},
					},
				},
			},
		},
		{
			name:    "vlan configuration with alias link name",
			cmdline: "vlan=vlan4095:enxa",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "enxa.4095",
						Logical:     true,
						Up:          true,
						Kind:        netconfig.LinkKindVLAN,
						Type:        nethelpers.LinkEther,
						ParentName:  "eth31",
						ConfigLayer: netconfig.ConfigCmdline,
						VLAN: netconfig.VLANSpec{
							VID:      4095,
							Protocol: nethelpers.VLANProtocol8021Q,
						},
					},
				},
			},
		},
		{
			name:          "vlan configuration with invalid vlan ID 4096",
			cmdline:       "vlan=eth1.4096:eth1",
			expectedError: fmt.Sprintf("invalid vlanID=%d, must be in the range 1..4095: %s", 4096, "eth1.4096:eth1"),
		},
		{
			name:          "vlan configuration with invalid vlan ID 0",
			cmdline:       "vlan=eth1.0:eth1",
			expectedError: fmt.Sprintf("invalid vlanID=%d, must be in the range 1..4095: %s", 0, "eth1.0:eth1"),
		},
		{
			name:    "multiple ip configurations",
			cmdline: "ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1::::: ip=eth3:dhcp ip=:::::eth4:dhcp::::",

			expectedSettings: network.CmdlineNetworking{
				LinkConfigs: []network.CmdlineLinkConfig{
					{
						Address:  netip.MustParsePrefix("172.20.0.2/24"),
						Gateway:  netip.MustParseAddr("172.20.0.1"),
						LinkName: "eth1",
					},
					{
						LinkName: "eth3",
						DHCP:     true,
					},
					{
						LinkName: "eth4",
						DHCP:     true,
					},
				},
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "eth1",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
					{
						Name:        "eth3",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
					{
						Name:        "eth4",
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmdline := procfs.NewCmdline(test.cmdline)

			link1 := netconfig.NewLinkStatus(netconfig.NamespaceName, "eth31")
			link1.TypedSpec().Alias = "enxa"
			link2 := netconfig.NewLinkStatus(netconfig.NamespaceName, "eth32")
			link2.TypedSpec().Alias = "enxb"

			settings, err := network.ParseCmdlineNetwork(
				cmdline,
				netconfig.NewLinkResolver(
					func() iter.Seq[*netconfig.LinkStatus] {
						return slices.Values([]*netconfig.LinkStatus{link1, link2})
					},
				),
			)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedSettings, settings)
			}
		})
	}
}
