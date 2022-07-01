// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	netconfig "github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type CmdlineSuite struct {
	suite.Suite
}

func (suite *CmdlineSuite) TestParse() {
	ifaces, _ := net.Interfaces() //nolint:errcheck // ignoring error here as ifaces will be empty

	sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name < ifaces[j].Name })

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
				Name:        "bond0",
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
					MasterName: "bond0",
					SlaveIndex: 0,
				},
			},
			{
				Name:        "eth1",
				Up:          true,
				Logical:     false,
				ConfigLayer: netconfig.ConfigCmdline,
				BondSlave: netconfig.BondSlave{
					MasterName: "bond0",
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
				Address:  netaddr.MustParseIPPrefix("172.20.0.2/24"),
				Gateway:  netaddr.MustParseIP("172.20.0.1"),
				LinkName: "eth1",
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
				Address:  netaddr.MustParseIPPrefix("172.20.0.2/32"),
				Gateway:  netaddr.MustParseIP("172.20.0.1"),
				LinkName: defaultIfaceName,
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
			name:    "complete",
			cmdline: "ip=172.20.0.2:172.21.0.1:172.20.0.1:255.255.255.0:master1:eth1::10.0.0.1:10.0.0.2:10.0.0.1",

			expectedSettings: network.CmdlineNetworking{
				Address:      netaddr.MustParseIPPrefix("172.20.0.2/24"),
				Gateway:      netaddr.MustParseIP("172.20.0.1"),
				Hostname:     "master1",
				LinkName:     "eth1",
				DNSAddresses: []netaddr.IP{netaddr.MustParseIP("10.0.0.1"), netaddr.MustParseIP("10.0.0.2")},
				NTPAddresses: []netaddr.IP{netaddr.MustParseIP("10.0.0.1")},
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
			name:    "ipv6",
			cmdline: "ip=[2001:db8::a]:[2001:db8::b]:[fe80::1]::master1:eth1::[2001:4860:4860::6464]:[2001:4860:4860::64]:[2001:4860:4806::]",
			expectedSettings: network.CmdlineNetworking{
				Address:      netaddr.MustParseIPPrefix("2001:db8::a/128"),
				Gateway:      netaddr.MustParseIP("fe80::1"),
				Hostname:     "master1",
				LinkName:     "eth1",
				DNSAddresses: []netaddr.IP{netaddr.MustParseIP("2001:4860:4860::6464"), netaddr.MustParseIP("2001:4860:4860::64")},
				NTPAddresses: []netaddr.IP{netaddr.MustParseIP("2001:4860:4806::")},
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
			name:    "unparseable IP",
			cmdline: "ip=xyz:",

			expectedError: "cmdline address parse failure: ParseIP(\"xyz\"): unable to parse IP",
		},
		{
			name:    "hostname override",
			cmdline: "ip=::::master1:eth1 talos.hostname=master2",

			expectedSettings: network.CmdlineNetworking{
				Hostname: "master2",
				LinkName: "eth1",
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
			cmdline: "talos.network.interface.ignore=eth2 talos.network.interface.ignore=eth3",

			expectedSettings: network.CmdlineNetworking{
				IgnoreInterfaces: []string{"eth2", "eth3"},
			},
		},
		{
			name:             "bond with no interfaces and no options set",
			cmdline:          "bond=bond0",
			expectedSettings: defaultBondSettings,
		},
		{
			name:             "bond with no interfaces and empty options set",
			cmdline:          "bond=bond0:::",
			expectedSettings: defaultBondSettings,
		},
		{
			name:    "bond with interfaces and no options set",
			cmdline: "bond=bond1:eth3,eth4",
			expectedSettings: network.CmdlineNetworking{
				NetworkLinkSpecs: []netconfig.LinkSpecSpec{
					{
						Name:        "bond1",
						Kind:        "bond",
						Type:        nethelpers.LinkEther,
						Logical:     true,
						Up:          true,
						ConfigLayer: netconfig.ConfigCmdline,
						BondMaster: netconfig.BondMasterSpec{
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
			cmdline: "vlan=eth1.5:eth1 ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1.5:::::",
			expectedSettings: network.CmdlineNetworking{
				Address:  netaddr.MustParseIPPrefix("172.20.0.2/24"),
				Gateway:  netaddr.MustParseIP("172.20.0.1"),
				LinkName: "eth1.5",
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
	} {
		test := test

		suite.Run(test.name, func() {
			cmdline := procfs.NewCmdline(test.cmdline)

			settings, err := network.ParseCmdlineNetwork(cmdline)

			if test.expectedError != "" {
				suite.Assert().EqualError(err, test.expectedError)
			} else {
				suite.Assert().NoError(err)
				suite.Assert().Equal(test.expectedSettings, settings)
			}
		})
	}
}

func TestCmdlineSuite(t *testing.T) {
	suite.Run(t, new(CmdlineSuite))
}
