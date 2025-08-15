// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"os"
	"os/exec"
	"slices"
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

func (s *NfTablesChainSuite) checkNftOutput(expected ...string) {
	s.T().Helper()

	var prevOutput string

	s.Eventually(func() bool {
		output := s.nftOutput()
		matches := slices.Contains(expected, strings.TrimSpace(output))

		if output != prevOutput {
			if !matches {
				s.T().Logf("nft list table inet talos-test:\n%s", output)
				s.T().Logf("expected:\n%s", strings.Join(expected, "\n"))
			}

			prevOutput = output
		}

		return matches
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
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchOIfName: &network.NfTablesIfNameMatch{
				InterfaceNames: []string{"lo"},
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

func (s *NfTablesChainSuite) TestAcceptMultipleIfnames() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPrioritySecurity
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchIIfName: &network.NfTablesIfNameMatch{
				InterfaceNames: []string{"eth0", "eth1"},
			},
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	// this seems to be a bug in the nft cli, it doesn't decoded the ifname anonymous set correctly
	// it might be that google/nftables doesn't set some magic on the anonymous set for the nft CLI to pick it up (?)
	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority security; policy accept;
		iifname { "", "" } accept
	}
}`)
}

func (s *NfTablesChainSuite) TestPolicyDrop() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPrioritySecurity
	chain.TypedSpec().Policy = nethelpers.VerdictDrop
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority security; policy drop;
		accept
	}
}`)
}

func (s *NfTablesChainSuite) TestICMPLimit() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPrioritySecurity
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchLayer4: &network.NfTablesLayer4Match{
				Protocol: nethelpers.ProtocolICMP,
			},
			MatchLimit: &network.NfTablesLimitMatch{
				PacketRatePerSecond: 5,
			},
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority security; policy accept;
		meta l4proto icmp limit rate 5/second burst 5 packets accept
	}
}`)
}

func (s *NfTablesChainSuite) TestConntrackCounter() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPrioritySecurity
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchConntrackState: &network.NfTablesConntrackStateMatch{
				States: []nethelpers.ConntrackState{
					nethelpers.ConntrackStateEstablished,
					nethelpers.ConntrackStateRelated,
				},
			},
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
		{
			MatchConntrackState: &network.NfTablesConntrackStateMatch{
				States: []nethelpers.ConntrackState{
					nethelpers.ConntrackStateInvalid,
				},
			},
			AnonCounter: true,
			Verdict:     pointer.To(nethelpers.VerdictDrop),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority security; policy accept;
		ct state { established, related } accept
		ct state invalid counter packets 0 bytes 0 drop
	}
}`)
}

func (s *NfTablesChainSuite) TestMatchMarksSubnets() {
	chain1 := network.NewNfTablesChain(network.NamespaceName, "test1")
	chain1.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain1.TypedSpec().Hook = nethelpers.ChainHookInput
	chain1.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain1.TypedSpec().Policy = nethelpers.VerdictAccept
	chain1.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchMark: &network.NfTablesMark{
				Mask:  constants.KubeSpanDefaultFirewallMask,
				Value: constants.KubeSpanDefaultFirewallMark,
			},
			MatchSourceAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
					netip.MustParsePrefix("0::/0"),
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
	chain2.TypedSpec().Policy = nethelpers.VerdictAccept
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
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
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
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			ClampMSS: &network.NfTablesClampMSS{
				MTU: constants.KubeSpanLinkMTU,
			},
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	// several versions here for different version of `nft` CLI decoding the rules
	s.checkNftOutput(`table inet talos-test {
	chain test1 {
		type filter hook input priority filter; policy accept;
		meta nfproto ipv4 tcp flags syn / syn,rst tcp option maxseg size > 1380 tcp option maxseg size set 1380
		meta nfproto ipv6 tcp flags syn / syn,rst tcp option maxseg size > 1360 tcp option maxseg size set 1360
	}
}`, `table inet talos-test {
	chain test1 {
		type filter hook input priority filter; policy accept;
		meta nfproto ipv4 tcp flags & (syn | rst) == syn tcp option maxseg size > 1380 tcp option maxseg size set 1380
		meta nfproto ipv6 tcp flags & (syn | rst) == syn tcp option maxseg size > 1360 tcp option maxseg size set 1360
	}
}`)
}

func (s *NfTablesChainSuite) TestL4Match() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test-tcp")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
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
		ip daddr { 10.0.0.0/8 } tcp dport { 1023-1025, 1027-1029 } drop
		ip6 daddr { 2001::/16 } tcp dport { 1023-1025, 1027-1029 } drop
	}
}`)
}

func (s *NfTablesChainSuite) TestL4Match2() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test-tcp")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchSourceAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
				},
				Invert: true,
			},
			MatchLayer4: &network.NfTablesLayer4Match{
				Protocol: nethelpers.ProtocolTCP,
				MatchDestinationPort: &network.NfTablesPortMatch{
					Ranges: []network.PortRange{
						{
							Lo: 1023,
							Hi: 1023,
						},
						{
							Lo: 1024,
							Hi: 1024,
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
		ip saddr != { 10.0.0.0/8 } tcp dport { 1023-1024 } drop
		meta nfproto ipv6 tcp dport { 1023-1024 } drop
	}
}`)
}

func (s *NfTablesChainSuite) TestL4MatchAdjacentPorts() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test-tcp")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchSourceAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("10.0.0.0/8"),
				},
				Invert: true,
			},
			MatchLayer4: &network.NfTablesLayer4Match{
				Protocol: nethelpers.ProtocolTCP,
				MatchDestinationPort: &network.NfTablesPortMatch{
					Ranges: []network.PortRange{
						{
							Lo: 5000,
							Hi: 5000,
						},
						{
							Lo: 5001,
							Hi: 5001,
						},
						{
							Lo: 10250,
							Hi: 10250,
						},
						{
							Lo: 4240,
							Hi: 4240,
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
		ip saddr != { 10.0.0.0/8 } tcp dport { 4240, 5000-5001, 10250 } drop
		meta nfproto ipv6 tcp dport { 4240, 5000-5001, 10250 } drop
	}
}`)
}

func (s *NfTablesChainSuite) TestL4MatchAny() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test-tcp")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchSourceAddress: &network.NfTablesAddressMatch{
				IncludeSubnets: []netip.Prefix{
					netip.MustParsePrefix("0.0.0.0/0"),
				},
			},
			MatchLayer4: &network.NfTablesLayer4Match{
				Protocol: nethelpers.ProtocolTCP,
				MatchDestinationPort: &network.NfTablesPortMatch{
					Ranges: []network.PortRange{
						{
							Lo: 1023,
							Hi: 1023,
						},
					},
				},
			},
			Verdict: pointer.To(nethelpers.VerdictAccept),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test-tcp {
		type filter hook input priority filter; policy accept;
		meta nfproto ipv4 tcp dport { 1023 } accept
	}
}`)
}

func (s *NfTablesChainSuite) TestICMPTypeMatch() {
	chain := network.NewNfTablesChain(network.NamespaceName, "test-tcp")
	chain.TypedSpec().Type = nethelpers.ChainTypeFilter
	chain.TypedSpec().Hook = nethelpers.ChainHookInput
	chain.TypedSpec().Priority = nethelpers.ChainPriorityFilter
	chain.TypedSpec().Policy = nethelpers.VerdictAccept
	chain.TypedSpec().Rules = []network.NfTablesRule{
		{
			MatchLayer4: &network.NfTablesLayer4Match{
				Protocol: nethelpers.ProtocolICMP,
				MatchICMPType: &network.NfTablesICMPTypeMatch{
					Types: []nethelpers.ICMPType{
						nethelpers.ICMPTypeTimestampRequest,
						nethelpers.ICMPTypeTimestampReply,
						nethelpers.ICMPTypeAddressMaskRequest,
						nethelpers.ICMPTypeAddressMaskReply,
					},
				},
			},
			AnonCounter: true,
			Verdict:     pointer.To(nethelpers.VerdictDrop),
		},
	}

	s.Require().NoError(s.State().Create(s.Ctx(), chain))

	s.checkNftOutput(`table inet talos-test {
	chain test-tcp {
		type filter hook input priority filter; policy accept;
		icmp type { timestamp-request, timestamp-reply, address-mask-request, address-mask-reply } counter packets 0 bytes 0 drop
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
