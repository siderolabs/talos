// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go4.org/netipx"

	"github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestNfTablesRuleCompile(t *testing.T) { //nolint:tparallel
	t.Parallel()

	for _, test := range []struct {
		name string

		spec networkres.NfTablesRule

		expectedRules [][]expr.Any
		expectedSets  []network.NfTablesSet
	}{
		{
			name: "empty",
		},
		{
			name: "match oifname",
			spec: networkres.NfTablesRule{
				MatchOIfName: &networkres.NfTablesIfNameMatch{
					InterfaceNames: []string{"eth0"},
					Operator:       nethelpers.OperatorEqual,
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte("eth0\000\000\000\000\000\000\000\000\000\000\000\000"),
					},
				},
			},
		},
		{
			name: "match iifname",
			spec: networkres.NfTablesRule{
				MatchIIfName: &networkres.NfTablesIfNameMatch{
					InterfaceNames: []string{"lo"},
					Operator:       nethelpers.OperatorNotEqual,
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpNeq,
						Register: 1,
						Data:     []byte("lo\000\000\000\000\000\000\000\000\000\000\000\000\000\000"),
					},
				},
			},
		},
		{
			name: "match multiple iifname",
			spec: networkres.NfTablesRule{
				MatchIIfName: &networkres.NfTablesIfNameMatch{
					InterfaceNames: []string{"siderolink", "kubespan"},
					Operator:       nethelpers.OperatorEqual,
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind: network.SetKindIfName,
					Strings: [][]byte{
						[]byte("siderolink\000\000\000\000\000\000"),
						[]byte("kubespan\000\000\000\000\000\000\000\000"),
					},
				},
			},
		},
		{
			name: "verdict accept",
			spec: networkres.NfTablesRule{
				MatchOIfName: &networkres.NfTablesIfNameMatch{
					InterfaceNames: []string{"eth0"},
					Operator:       nethelpers.OperatorNotEqual,
				},
				Verdict: pointer.To(nethelpers.VerdictAccept),
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpNeq,
						Register: 1,
						Data:     []byte("eth0\000\000\000\000\000\000\000\000\000\000\000\000"),
					},
					&expr.Verdict{Kind: expr.VerdictAccept},
				},
			},
		},
		{
			name: "match and set mark",
			spec: networkres.NfTablesRule{
				MatchMark: &networkres.NfTablesMark{
					Mask:  0xff00ffff,
					Xor:   0x00ff0000,
					Value: 0x00ee0000,
				},
				SetMark: &networkres.NfTablesMark{
					Mask: 0x0000ffff,
					Xor:  0xffff0000,
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyMARK, Register: 1},
					&expr.Bitwise{
						SourceRegister: 1,
						DestRegister:   1,
						Len:            4,
						Xor:            []byte{0x00, 0x00, 0xff, 0x00},
						Mask:           []byte{0xff, 0xff, 0x00, 0xff},
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{0x00, 0x00, 0xee, 0x00},
					},
					&expr.Meta{Key: expr.MetaKeyMARK, Register: 1},
					&expr.Bitwise{
						SourceRegister: 1,
						DestRegister:   1,
						Len:            4,
						Xor:            []byte{0x00, 0x00, 0xff, 0xff},
						Mask:           []byte{0xff, 0xff, 0x00, 0x00},
					},
					&expr.Meta{Key: expr.MetaKeyMARK, SourceRegister: true, Register: 1},
				},
			},
		},
		{
			name: "match on empty source address",
			spec: networkres.NfTablesRule{
				MatchSourceAddress: &networkres.NfTablesAddressMatch{},
				Verdict:            pointer.To(nethelpers.VerdictDrop),
			},
		},
		{
			name: "match on v4 source address",
			spec: networkres.NfTablesRule{
				MatchSourceAddress: &networkres.NfTablesAddressMatch{
					IncludeSubnets: []netip.Prefix{
						netip.MustParsePrefix("192.168.0.0/16"),
					},
					ExcludeSubnets: []netip.Prefix{
						netip.MustParsePrefix("192.168.4.0/24"),
					},
				},
				Verdict: pointer.To(nethelpers.VerdictDrop),
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv4)},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseNetworkHeader,
						Offset:       12,
						Len:          4,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
					},
					&expr.Verdict{
						Kind: expr.VerdictDrop,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind:      network.SetKindIPv4,
					Addresses: []netipx.IPRange{netipx.MustParseIPRange("192.168.0.0-192.168.3.255"), netipx.MustParseIPRange("192.168.5.0-192.168.255.255")},
				},
			},
		},
		{
			name: "match on v6 source and destination addresses",
			spec: networkres.NfTablesRule{
				MatchSourceAddress: &networkres.NfTablesAddressMatch{
					IncludeSubnets: []netip.Prefix{
						netip.MustParsePrefix("2001::/16"),
					},
				},
				MatchDestinationAddress: &networkres.NfTablesAddressMatch{
					IncludeSubnets: []netip.Prefix{
						netip.MustParsePrefix("20fe::/16"),
					},
					Invert: true,
				},
				Verdict: pointer.To(nethelpers.VerdictDrop),
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv4)},
					},
					&expr.Verdict{
						Kind: expr.VerdictDrop,
					},
				},
				{
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv6)},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseNetworkHeader,
						Offset:       8,
						Len:          16,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseNetworkHeader,
						Offset:       24,
						Len:          16,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          1,
						Invert:         true,
					},
					&expr.Verdict{
						Kind: expr.VerdictDrop,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind:      network.SetKindIPv6,
					Addresses: []netipx.IPRange{netipx.MustParseIPRange("2001::-2001:ffff:ffff:ffff:ffff:ffff:ffff:ffff")},
				},
				{
					Kind:      network.SetKindIPv6,
					Addresses: []netipx.IPRange{netipx.MustParseIPRange("20fe::-20fe:ffff:ffff:ffff:ffff:ffff:ffff:ffff")},
				},
			},
		},
		{
			name: "match on v6 destination addresses",
			spec: networkres.NfTablesRule{
				MatchDestinationAddress: &networkres.NfTablesAddressMatch{
					IncludeSubnets: []netip.Prefix{
						netip.MustParsePrefix("20fe::/16"),
					},
				},
				Verdict: pointer.To(nethelpers.VerdictDrop),
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv6)},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseNetworkHeader,
						Offset:       24,
						Len:          16,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
					},
					&expr.Verdict{
						Kind: expr.VerdictDrop,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind:      network.SetKindIPv6,
					Addresses: []netipx.IPRange{netipx.MustParseIPRange("20fe::-20fe:ffff:ffff:ffff:ffff:ffff:ffff:ffff")},
				},
			},
		},
		{
			name: "match on any v6 address",
			spec: networkres.NfTablesRule{
				MatchSourceAddress: &networkres.NfTablesAddressMatch{
					IncludeSubnets: []netip.Prefix{
						netip.MustParsePrefix("192.168.37.45/32"),
					},
					Invert: true,
				},
				Verdict: pointer.To(nethelpers.VerdictDrop),
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv4)},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseNetworkHeader,
						Offset:       12,
						Len:          4,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
						Invert:         true,
					},
					&expr.Verdict{
						Kind: expr.VerdictDrop,
					},
				},
				{
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv6)},
					},
					&expr.Verdict{
						Kind: expr.VerdictDrop,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind:      network.SetKindIPv4,
					Addresses: []netipx.IPRange{netipx.MustParseIPRange("192.168.37.45-192.168.37.45")},
				},
			},
		},
		{
			name: "clamp MSS",
			spec: networkres.NfTablesRule{
				ClampMSS: &networkres.NfTablesClampMSS{
					MTU: 1280,
				},
			},
			expectedRules: [][]expr.Any{
				{ //nolint:dupl
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv4)},
					},
					&expr.Meta{
						Key:      expr.MetaKeyL4PROTO,
						Register: 1,
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{6},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseTransportHeader,
						Offset:       13,
						Len:          1,
					},
					&expr.Bitwise{
						DestRegister:   1,
						SourceRegister: 1,
						Len:            1,
						Mask:           []byte{0x02 | 0x04},
						Xor:            []byte{0x00},
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{0x02},
					},
					&expr.Exthdr{
						DestRegister: 1,
						Type:         2,
						Offset:       2,
						Len:          2,
						Op:           expr.ExthdrOpTcpopt,
					},
					&expr.Cmp{
						Op:       expr.CmpOpGt,
						Register: 1,
						Data:     []byte{0x04, 0xd8},
					},
					&expr.Immediate{
						Register: 1,
						Data:     []byte{0x04, 0xd8},
					},
					&expr.Exthdr{
						SourceRegister: 1,
						Type:           2,
						Offset:         2,
						Len:            2,
						Op:             expr.ExthdrOpTcpopt,
					},
				},
				{ //nolint:dupl
					&expr.Meta{Key: expr.MetaKeyNFPROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{byte(nftables.TableFamilyIPv6)},
					},
					&expr.Meta{
						Key:      expr.MetaKeyL4PROTO,
						Register: 1,
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{6},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseTransportHeader,
						Offset:       13,
						Len:          1,
					},
					&expr.Bitwise{
						DestRegister:   1,
						SourceRegister: 1,
						Len:            1,
						Mask:           []byte{0x02 | 0x04},
						Xor:            []byte{0x00},
					},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{0x02},
					},
					&expr.Exthdr{
						DestRegister: 1,
						Type:         2,
						Offset:       2,
						Len:          2,
						Op:           expr.ExthdrOpTcpopt,
					},
					&expr.Cmp{
						Op:       expr.CmpOpGt,
						Register: 1,
						Data:     []byte{0x04, 0xc4},
					},
					&expr.Immediate{
						Register: 1,
						Data:     []byte{0x04, 0xc4},
					},
					&expr.Exthdr{
						SourceRegister: 1,
						Type:           2,
						Offset:         2,
						Len:            2,
						Op:             expr.ExthdrOpTcpopt,
					},
				},
			},
		},
		{
			name: "match L4 proto",
			spec: networkres.NfTablesRule{
				MatchLayer4: &networkres.NfTablesLayer4Match{
					Protocol: nethelpers.ProtocolUDP,
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{0x11},
					},
				},
			},
		},
		{
			name: "match L4 proto and src port",
			spec: networkres.NfTablesRule{
				MatchLayer4: &networkres.NfTablesLayer4Match{
					Protocol: nethelpers.ProtocolTCP,
					MatchSourcePort: &networkres.NfTablesPortMatch{
						Ranges: []networkres.PortRange{
							{
								Lo: 2000,
								Hi: 2000,
							},
							{
								Lo: 1000,
								Hi: 1025,
							},
						},
					},
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{0x6},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseTransportHeader,
						Offset:       0,
						Len:          2,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind: network.SetKindPort,
					Ports: [][2]uint16{
						{2000, 2000},
						{1000, 1025},
					},
				},
			},
		},
		{
			name: "match L4 proto and dst port",
			spec: networkres.NfTablesRule{
				MatchLayer4: &networkres.NfTablesLayer4Match{
					Protocol: nethelpers.ProtocolTCP,
					MatchDestinationPort: &networkres.NfTablesPortMatch{
						Ranges: []networkres.PortRange{
							{
								Lo: 2000,
								Hi: 2000,
							},
						},
					},
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
					&expr.Cmp{
						Op:       expr.CmpOpEq,
						Register: 1,
						Data:     []byte{0x6},
					},
					&expr.Payload{
						DestRegister: 1,
						Base:         expr.PayloadBaseTransportHeader,
						Offset:       2,
						Len:          2,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind: network.SetKindPort,
					Ports: [][2]uint16{
						{2000, 2000},
					},
				},
			},
		},
		{
			name: "limit",
			spec: networkres.NfTablesRule{
				MatchLimit: &networkres.NfTablesLimitMatch{
					PacketRatePerSecond: 5,
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Limit{
						Type:  expr.LimitTypePkts,
						Rate:  5,
						Burst: 5,
						Unit:  expr.LimitTimeSecond,
					},
				},
			},
		},
		{
			name: "counter",
			spec: networkres.NfTablesRule{
				AnonCounter: true,
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Counter{},
				},
			},
		},
		{
			name: "ct state",
			spec: networkres.NfTablesRule{
				MatchConntrackState: &networkres.NfTablesConntrackStateMatch{
					States: []nethelpers.ConntrackState{
						nethelpers.ConntrackStateInvalid,
					},
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Ct{
						Key:      expr.CtKeySTATE,
						Register: 1,
					},
					&expr.Bitwise{
						DestRegister:   1,
						SourceRegister: 1,
						Len:            4,
						Mask:           []byte{0x01, 0x00, 0x00, 0x00},
						Xor:            []byte{0x00, 0x00, 0x00, 0x00},
					},
					&expr.Cmp{
						Op:       expr.CmpOpNeq,
						Register: 1,
						Data:     []byte{0x00, 0x00, 0x00, 0x00},
					},
				},
			},
		},
		{
			name: "ct states",
			spec: networkres.NfTablesRule{
				MatchConntrackState: &networkres.NfTablesConntrackStateMatch{
					States: []nethelpers.ConntrackState{
						nethelpers.ConntrackStateRelated,
						nethelpers.ConntrackStateEstablished,
					},
				},
			},
			expectedRules: [][]expr.Any{
				{
					&expr.Ct{
						Key:      expr.CtKeySTATE,
						Register: 1,
					},
					&expr.Lookup{
						SourceRegister: 1,
						SetID:          0,
					},
				},
			},
			expectedSets: []network.NfTablesSet{
				{
					Kind: network.SetKindConntrackState,
					ConntrackStates: []nethelpers.ConntrackState{
						nethelpers.ConntrackStateRelated,
						nethelpers.ConntrackStateEstablished,
					},
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := network.NfTablesRule(&test.spec).Compile()
			require.NoError(t, err)

			assert.Equal(t, test.expectedRules, result.Rules)
			assert.Equal(t, test.expectedSets, result.Sets)
		})
	}
}

func TestNftablesSet(t *testing.T) { //nolint:tparallel
	t.Parallel()

	for _, test := range []struct {
		name string

		set network.NfTablesSet

		expectedKeyType  nftables.SetDatatype
		expectedInterval bool
		expectedData     []nftables.SetElement
	}{
		{
			name: "ports",

			set: network.NfTablesSet{
				Kind: network.SetKindPort,
				Ports: [][2]uint16{
					{443, 443},
					{80, 81},
					{5000, 5000},
					{5001, 5001},
				},
			},

			expectedKeyType:  nftables.TypeInetService,
			expectedInterval: true,
			expectedData: []nftables.SetElement{ // network byte order
				{Key: []uint8{0x0, 80}, IntervalEnd: false}, // 80 - 81
				{Key: []uint8{0x0, 82}, IntervalEnd: true},
				{Key: []uint8{0x1, 0xbb}, IntervalEnd: false}, // 443-443
				{Key: []uint8{0x1, 0xbc}, IntervalEnd: true},
				{Key: []uint8{0x13, 0x88}, IntervalEnd: false}, // 5000-5001
				{Key: []uint8{0x13, 0x8a}, IntervalEnd: true},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedKeyType, test.set.KeyType())
			assert.Equal(t, test.expectedInterval, test.set.IsInterval())
			assert.Equal(t, test.expectedData, test.set.SetElements())
		})
	}
}
