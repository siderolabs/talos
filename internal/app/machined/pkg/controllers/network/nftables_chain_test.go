// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type NfTablesChainSuite struct {
	ctest.DefaultSuite
}

func (s *NfTablesChainSuite) nftOutput() string {
	out, err := exec.Command("nft", "list", "table", "inet", "talos-test").CombinedOutput()
	s.Require().NoError(err, "nft list table inet talos-test failed: %s", string(out))

	return string(out)
}

func (s *NfTablesChainSuite) checkNftOutput(expected string) {
	s.T().Helper()

	var prevOutput string

	s.Eventually(func() bool {
		output := s.nftOutput()

		if output != prevOutput {
			if strings.TrimSpace(output) != expected {
				s.T().Logf("nft list table inet talos-test:\n%s", output)
			}

			prevOutput = output
		}

		return strings.TrimSpace(output) == expected
	}, 5*time.Second, 100*time.Millisecond)
}

func (s *NfTablesChainSuite) TestEmpty() {
	s.checkNftOutput(`table inet talos-test {
}`)
}

func (s *NfTablesChainSuite) TestAcceptLo() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPrioritySecurity
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchOIfName: &network.NfTablesIfNameMatch{
				InterfaceName: "lo",
			},
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority security; policy accept;
		oifname "lo" accept
	}
}`)
}

func (s *NfTablesChainSuite) TestMatchMarksSubnets() {
	chain1 := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain1.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain1.TypedSpec().Hook = nethelpers.ChainHookInput
	chain1.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain1.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchMark: &network.NfTablesMark{
				Mask:  constants.KubeSpanDefaultFirewallMask,
				Value: constants.KubeSpanDefaultFirewallMark,
			},
			MatchSourceAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
				},
				ExcludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.3.0.0/16"),
				},
				Invert: true,
			},
			MatchDestinationAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("192.168.0.0/24"),
				},
			},
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain1))

	chain2 := network.NewNfTablesChain(network.NamespaceName, "test2")
	chain2.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain2.TypedSpec().Hook = nethelpers.ChainHookInput
	chain2.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain2.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchDestinationAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("192.168.3.5/32"),
				},
			},
			SetMark: &network.NfTablesMark{
				Mask: ^uint32(constants.KubeSpanDefaultFirewallMask),
				Xor:  constants.KubeSpanDefaultFirewallMark,
			},
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain2))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority filter; policy accept;
		meta mark & 0x00000060 == 0x00000020 ip saddr != { 10.0.0.0-10.2.255.255, 10.4.0.0-10.255.255.255 } ip daddr { 192.168.0.0/24 } accept
		meta mark & 0x00000060 == 0x00000020 meta nfproto ipv6 accept
	}

	chain test2 {
		type filter hook input priority filter; policy accept;
		ip daddr { 192.168.3.5 } meta mark set meta mark & 0xffffffbf | 0x00000020
	}
}`)
}

func (s *NfTablesChainSuite) TestUpdateChains() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchSourceAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
				},
				ExcludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.3.0.0/16"),
				},
				Invert: true,
			},
			MatchDestinationAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("192.168.0.0/24"),
				},
			},
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority filter; policy accept;
		ip saddr != { 10.0.0.0-10.2.255.255, 10.4.0.0-10.255.255.255 } ip daddr { 192.168.0.0/24 } accept
		meta nfproto ipv6 accept
	}
}`)

	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchSourceAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
				},
				ExcludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.4.0.0/16"),
				},
				Invert: true,
			},
			SetMark: &network.NfTablesMark{
				Mask: ^uint32(constants.KubeSpanDefaultFirewallMask),
				Xor:  constants.KubeSpanDefaultFirewallMark,
			},
		},
	}

	s.Require().NoError(s.State().Update(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority filter; policy accept;
		ip saddr != { 10.0.0.0/14, 10.5.0.0-10.255.255.255 } meta mark set meta mark & 0xffffffbf | 0x00000020
		meta nfproto ipv6 meta mark set meta mark & 0xffffffbf | 0x00000020
	}
}`)

	s.Require().NoError(s.State().Destroy(s.Ctx(), chain.Metadata()))

	s.checkNftOutput(`table inet talos-test {
}`)
}

func (s *NfTablesChainSuite) TestClampMSS() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			ClampMSS: &network.NfTablesClampMSS{
				MTU: constants.KubeSpanLinkMTU,
			},
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority filter; policy accept;
		meta nfproto ipv4 tcp flags syn / syn,rst tcp option maxseg size > 1380 tcp option maxseg size set 1380
		meta nfproto ipv6 tcp flags syn / syn,rst tcp option maxseg size > 1360 tcp option maxseg size set 1360
	}
}`)
}

func (s *NfTablesChainSuite) TestL4Match() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test-tcp")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchDestinationAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
					netip.MustParsePrefix("2001::/16"),
				},
			},
			MatchLayer4: &network.NfTablesLayer4Match{
				Protocol: nethelpers.ProtocolTCP,
				MatchDestinationPort: &network.NfTablesPortMatch{
					Ranges: []network.PortRange{
						{
							Lo: 1023,
							Hi: 1025,
						},
						{
							Lo: 1027,
							Hi: 1029,
						},
					},
				},
			},
			Verdict: pointer.To(nethelpers.VerdictDrop),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test-tcp {
		type filter hook input priority filter; policy accept;
		ip daddr { 10.0.0.0/8 } tcp dport { 1023-1024, 1027-1028 } drop
		ip6 daddr { 2001::/16 } tcp dport { 1023-1024, 1027-1028 } drop
	}
}`)
}

func TestNftablesChainSuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	if exec.Command("nft", "list", "tables").Run() != nil {
		t.Skip("requires nftables CLI to be installed")
	}

	suite.Run(t, &NfTablesChainSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				// try to see if the table is there
				if exec.Command("nft", "list", "table", "inet", "talos-test").Run() == nil {
					s.Require().NoError(exec.Command("nft", "delete", "table", "inet", "talos-test").Run())
				}

				s.Require().NoError(s.Runtime().RegisterController(&netctrl.NfTablesChainController{TableName: "talos-test"}))
			},
			AfterTearDown: func(s *ctest.DefaultSuite) {
				s.Require().NoError(exec.Command("nft", "delete", "table", "inet", "talos-test").Run())
			},
		},
	})
}
