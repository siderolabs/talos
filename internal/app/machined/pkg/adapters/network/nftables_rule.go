// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"cmp"
	"fmt"
	"net/netip"
	"os"
	"slices"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/siderolabs/gen/xslices"
	"go4.org/netipx"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NfTablesRule adapter provides encoding to nftables instructions.
//
//nolint:revive,golint
func NfTablesRule(r *network.NfTablesRule) nftablesRule {
	return nftablesRule{
		NfTablesRule: r,
	}
}

type nftablesRule struct {
	*network.NfTablesRule
}

// SetKind is the type of the nftables Set.
type SetKind uint8

// SetKind constants.
const (
	SetKindIPv4 SetKind = iota
	SetKindIPv6
	SetKindPort
	SetKindIfName
	SetKindConntrackState
)

// NfTablesSet is a compiled representation of the set.
type NfTablesSet struct {
	Kind            SetKind
	Addresses       []netipx.IPRange
	Ports           [][2]uint16
	Strings         [][]byte
	ConntrackStates []nethelpers.ConntrackState
}

// IsInterval returns true if the set is an interval set.
func (set NfTablesSet) IsInterval() bool {
	switch set.Kind {
	case SetKindIPv4, SetKindIPv6, SetKindPort:
		return true
	case SetKindIfName, SetKindConntrackState:
		return false
	default:
		panic(fmt.Sprintf("unknown set kind: %d", set.Kind))
	}
}

// KeyType returns the type of the set.
func (set NfTablesSet) KeyType() nftables.SetDatatype {
	switch set.Kind {
	case SetKindIPv4:
		return nftables.TypeIPAddr
	case SetKindIPv6:
		return nftables.TypeIP6Addr
	case SetKindPort:
		return nftables.TypeInetService
	case SetKindIfName:
		return nftables.TypeIFName
	case SetKindConntrackState:
		return nftables.TypeCTState
	default:
		panic(fmt.Sprintf("unknown set kind: %d", set.Kind))
	}
}

// SetElements returns the set elements.
func (set NfTablesSet) SetElements() []nftables.SetElement {
	switch set.Kind {
	case SetKindIPv4, SetKindIPv6:
		elements := make([]nftables.SetElement, 0, len(set.Addresses)*2)

		for _, r := range set.Addresses {
			fromBin, _ := r.From().MarshalBinary()    //nolint:errcheck // doesn't fail
			toBin, _ := r.To().Next().MarshalBinary() //nolint:errcheck // doesn't fail

			elements = append(elements,
				nftables.SetElement{
					Key:         fromBin,
					IntervalEnd: false,
				},
				nftables.SetElement{
					Key:         toBin,
					IntervalEnd: true,
				},
			)
		}

		return elements
	case SetKindPort:
		ports := mergeAdjacentPorts(set.Ports)

		elements := make([]nftables.SetElement, 0, len(ports))

		for _, p := range ports {
			from := binaryutil.BigEndian.PutUint16(p[0])
			to := binaryutil.BigEndian.PutUint16(p[1] + 1)

			elements = append(elements,
				nftables.SetElement{
					Key:         from,
					IntervalEnd: false,
				},
				nftables.SetElement{
					Key:         to,
					IntervalEnd: true,
				},
			)
		}

		return elements
	case SetKindIfName:
		elements := make([]nftables.SetElement, 0, len(set.Strings))

		for _, s := range set.Strings {
			elements = append(elements,
				nftables.SetElement{
					Key: s,
				},
			)
		}

		return elements
	case SetKindConntrackState:
		elements := make([]nftables.SetElement, 0, len(set.ConntrackStates))

		for _, s := range set.ConntrackStates {
			elements = append(elements,
				nftables.SetElement{
					Key: binaryutil.NativeEndian.PutUint32(uint32(s)),
				},
			)
		}

		return elements
	default:
		panic(fmt.Sprintf("unknown set kind: %d", set.Kind))
	}
}

func mergeAdjacentPorts(in [][2]uint16) [][2]uint16 {
	ports := slices.Clone(in)

	slices.SortFunc(ports, func(a, b [2]uint16) int {
		// sort by the lower bound of the range, assume no overlap
		return cmp.Compare(a[0], b[0])
	})

	for i := 0; i < len(ports)-1; {
		if ports[i][1]+1 >= ports[i+1][0] {
			ports[i][1] = ports[i+1][1]
			ports = append(ports[:i+1], ports[i+2:]...)
		} else {
			i++
		}
	}

	return ports
}

// NfTablesCompiled is a compiled representation of the rule.
type NfTablesCompiled struct {
	Rules [][]expr.Any
	Sets  []NfTablesSet
}

var (
	matchV4 = []expr.Any{
		// Store protocol type to register 1
		&expr.Meta{
			Key:      expr.MetaKeyNFPROTO,
			Register: 1,
		},
		// Match IP Family
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{byte(nftables.TableFamilyIPv4)},
		},
	}

	matchV6 = []expr.Any{
		// Store protocol type to register 1
		&expr.Meta{
			Key:      expr.MetaKeyNFPROTO,
			Register: 1,
		},
		// Match IP Family
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{byte(nftables.TableFamilyIPv6)},
		},
	}

	firstIPv4 = netip.MustParseAddr("0.0.0.0")
	lastIPv4  = netip.MustParseAddr("255.255.255.255")

	firstIPv6 = netip.MustParseAddr("::")
	lastIPv6  = netip.MustParseAddr("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")
)

// Compile translates the rule into the set of nftables instructions.
//
//nolint:gocyclo,cyclop
func (a nftablesRule) Compile() (*NfTablesCompiled, error) {
	var (
		// common for ipv4 & ipv6 expression, pre & post
		rulePre  []expr.Any
		rulePost []expr.Any
		// speficic for ipv4 & ipv6 expression
		rule4, rule6 []expr.Any

		result NfTablesCompiled
	)

	matchIfNames := func(operator nethelpers.MatchOperator, ifnames []string) {
		if len(ifnames) == 1 {
			rulePre = append(rulePre,
				// [ cmp eq/neq reg 1 <ifname> ]
				&expr.Cmp{
					Op:       expr.CmpOp(operator),
					Register: 1,
					Data:     ifname(ifnames[0]),
				},
			)
		} else {
			result.Sets = append(result.Sets,
				NfTablesSet{
					Kind:    SetKindIfName,
					Strings: xslices.Map(ifnames, ifname),
				})

			rulePre = append(rulePre,
				// Match from target set
				&expr.Lookup{
					SourceRegister: 1,
					SetID:          uint32(len(result.Sets) - 1), // reference will be fixed up by the controller
					Invert:         operator == nethelpers.OperatorNotEqual,
				},
			)
		}
	}

	if a.NfTablesRule.MatchIIfName != nil {
		match := a.NfTablesRule.MatchIIfName

		rulePre = append(rulePre,
			// [ meta load iifname => reg 1 ]
			&expr.Meta{
				Key:      expr.MetaKeyIIFNAME,
				Register: 1,
			},
		)

		matchIfNames(match.Operator, match.InterfaceNames)
	}

	if a.NfTablesRule.MatchOIfName != nil {
		match := a.NfTablesRule.MatchOIfName

		rulePre = append(rulePre,
			// [ meta load oifname => reg 1 ]
			&expr.Meta{
				Key:      expr.MetaKeyOIFNAME,
				Register: 1,
			},
		)

		matchIfNames(match.Operator, match.InterfaceNames)
	}

	if a.NfTablesRule.MatchMark != nil {
		match := a.NfTablesRule.MatchMark

		rulePre = append(rulePre,
			// [ meta load mark => reg 1 ]
			&expr.Meta{
				Key:      expr.MetaKeyMARK,
				Register: 1,
			},
			// Mask the mark with the configured mask:
			//  R1 = R1 & mask ^ xor
			&expr.Bitwise{
				SourceRegister: 1,
				DestRegister:   1,
				Len:            4,
				Xor:            binaryutil.NativeEndian.PutUint32(match.Xor),
				Mask:           binaryutil.NativeEndian.PutUint32(match.Mask),
			},
			// Compare the masked firewall mark with expected value
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     binaryutil.NativeEndian.PutUint32(match.Value),
			},
		)
	}

	if a.NfTablesRule.MatchConntrackState != nil {
		match := a.NfTablesRule.MatchConntrackState

		if len(match.States) == 1 {
			rulePre = append(rulePre,
				// [ ct load state => reg 1 ]
				&expr.Ct{
					Key:      expr.CtKeySTATE,
					Register: 1,
				},
				// [ bitwise reg 1 = ( reg 1 & state ) ^ 0x00000000 ]
				&expr.Bitwise{
					SourceRegister: 1,
					DestRegister:   1,
					Len:            4,
					Mask:           binaryutil.NativeEndian.PutUint32(uint32(match.States[0])),
					Xor:            []byte{0x0, 0x0, 0x0, 0x0},
				},
				// [ cmp neq reg 1 0x00000000 ]
				&expr.Cmp{
					Op:       expr.CmpOpNeq,
					Register: 1,
					Data:     []byte{0x0, 0x0, 0x0, 0x0},
				},
			)
		} else {
			result.Sets = append(result.Sets,
				NfTablesSet{
					Kind:            SetKindConntrackState,
					ConntrackStates: match.States,
				})

			rulePre = append(rulePre,
				// [ ct load state => reg 1 ]
				&expr.Ct{
					Key:      expr.CtKeySTATE,
					Register: 1,
				},
				// [ lookup reg 1 set <set> ]
				&expr.Lookup{
					SourceRegister: 1,
					SetID:          uint32(len(result.Sets) - 1), // reference will be fixed up by the controller
				},
			)
		}
	}

	addressMatchExpression := func(match *network.NfTablesAddressMatch, label string, offV4, offV6 uint32) error {
		ipSet, err := BuildIPSet(match.IncludeSubnets, match.ExcludeSubnets)
		if err != nil {
			return fmt.Errorf("failed to build IPSet for %s address match: %w", label, err)
		}

		v4Set, v6Set := SplitIPSet(ipSet)

		if v4Set == nil && v6Set == nil && !match.Invert {
			// this rule doesn't match anything
			return os.ErrNotExist
		}

		v4SetCoversAll := len(v4Set) == 1 && v4Set[0].From() == firstIPv4 && v4Set[0].To() == lastIPv4
		v6SetCoversAll := len(v6Set) == 1 && v6Set[0].From() == firstIPv6 && v6Set[0].To() == lastIPv6

		if v4SetCoversAll && v6SetCoversAll && match.Invert {
			// this rule doesn't match anything
			return os.ErrNotExist
		}

		switch { //nolint:dupl
		case v4SetCoversAll && !match.Invert, match.Invert && v4Set == nil:
			// match any v4 IP
			if rule4 == nil {
				rule4 = []expr.Any{}
			}
		case !v4SetCoversAll && match.Invert, !match.Invert && v4Set != nil:
			// match specific v4 IPs
			result.Sets = append(result.Sets,
				NfTablesSet{
					Kind:      SetKindIPv4,
					Addresses: v4Set,
				},
			)

			rule4 = append(rule4,
				// Store the destination IP address to register 1
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       offV4,
					Len:          4,
				},
				// Match from target set
				&expr.Lookup{
					SourceRegister: 1,
					SetID:          uint32(len(result.Sets) - 1), // reference will be fixed up by the controller
					Invert:         match.Invert,
				},
			)
		default: // otherwise skip generating v4 rule, as it doesn't match anything
		}

		switch { //nolint:dupl
		case v6SetCoversAll && !match.Invert, match.Invert && v6Set == nil:
			// match any v6 IP
			if rule6 == nil {
				rule6 = []expr.Any{}
			}
		case !v6SetCoversAll && match.Invert, !match.Invert && v6Set != nil:
			// match specific v6 IPs
			result.Sets = append(result.Sets,
				NfTablesSet{
					Kind:      SetKindIPv6,
					Addresses: v6Set,
				})

			rule6 = append(rule6,
				// Store the destination IP address to register 1
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       offV6,
					Len:          16,
				},
				// Match from target set
				&expr.Lookup{
					SourceRegister: 1,
					SetID:          uint32(len(result.Sets) - 1), // reference will be fixed up by the controller
					Invert:         match.Invert,
				},
			)
		default: // otherwise skip generating v6 rule, as it doesn't match anything
		}

		return nil
	}

	if a.NfTablesRule.MatchSourceAddress != nil {
		match := a.NfTablesRule.MatchSourceAddress

		if err := addressMatchExpression(match, "source", 12, 8); err != nil {
			if os.IsNotExist(err) {
				return &NfTablesCompiled{}, nil
			}

			return nil, err
		}
	}

	if a.NfTablesRule.MatchDestinationAddress != nil {
		match := a.NfTablesRule.MatchDestinationAddress

		if err := addressMatchExpression(match, "destination", 16, 24); err != nil {
			if os.IsNotExist(err) {
				return &NfTablesCompiled{}, nil
			}

			return nil, err
		}
	}

	if a.NfTablesRule.MatchLayer4 != nil {
		match := a.NfTablesRule.MatchLayer4

		rulePre = append(rulePre,
			// [ meta load l4proto => reg 1 ]
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: 1,
			},
			// [ cmp eq reg 1 <protocol> ]
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{byte(match.Protocol)},
			},
		)

		portMatch := func(off uint32, ports []network.PortRange) {
			result.Sets = append(result.Sets,
				NfTablesSet{
					Kind:  SetKindPort,
					Ports: xslices.Map(ports, func(r network.PortRange) [2]uint16 { return [2]uint16{r.Lo, r.Hi} }),
				},
			)

			rulePost = append(rulePost,
				// [ payload load 2b @ transport header + <offset> => reg 1 ]
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       off,
					Len:          2,
				},
				// [ lookup reg 1 set <set> ]
				&expr.Lookup{
					SourceRegister: 1,
					SetID:          uint32(len(result.Sets) - 1), // reference will be fixed up by the controller
				},
			)
		}

		if match.MatchSourcePort != nil {
			portMatch(0, match.MatchSourcePort.Ranges)
		}

		if match.MatchDestinationPort != nil {
			portMatch(2, match.MatchDestinationPort.Ranges)
		}
	}

	if a.NfTablesRule.MatchLimit != nil {
		match := a.NfTablesRule.MatchLimit

		rulePost = append(rulePost,
			// [ limit rate <rate> ]
			&expr.Limit{
				Type:  expr.LimitTypePkts,
				Rate:  match.PacketRatePerSecond,
				Burst: uint32(match.PacketRatePerSecond),
				Unit:  expr.LimitTimeSecond,
			},
		)
	}

	clampMSS := func(family nftables.TableFamily, mtu uint16) []expr.Any {
		var mss uint16

		switch family { //nolint:exhaustive
		case nftables.TableFamilyIPv4:
			mss = mtu - 40 // TCP + IPv4 overhead
		case nftables.TableFamilyIPv6:
			mss = mtu - 60 // TCP + IPv6 overhead
		default:
			panic("unexpected IP family")
		}

		return []expr.Any{
			// Load the L4 protocol into register 1
			&expr.Meta{
				Key:      expr.MetaKeyL4PROTO,
				Register: 1,
			},
			// Match TCP Family
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{unix.IPPROTO_TCP},
			},
			// [ payload load 1b @ transport header + 13 => reg 1 ]
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseTransportHeader,
				Offset:       13,
				Len:          1,
			},
			// [ bitwise reg 1 = ( reg 1 & 0x00000006 ) ^ 0x00000000 ]
			&expr.Bitwise{
				DestRegister:   1,
				SourceRegister: 1,
				Len:            1,
				Mask:           []byte{0x02 | 0x04},
				Xor:            []byte{0x00},
			},
			// [ cmp eq reg 1 0x00000002 ]
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte{0x02},
			},
			// [ exthdr load tcpopt 2b @ 2 + 2 => reg 1 ]
			&expr.Exthdr{
				DestRegister: 1,
				Type:         2,
				Offset:       2,
				Len:          2,
				Op:           expr.ExthdrOpTcpopt,
			},
			// [ cmp gte reg 1 MTU ]
			&expr.Cmp{
				Op:       expr.CmpOpGt,
				Register: 1,
				Data:     binaryutil.BigEndian.PutUint16(mss),
			},
			// [ immediate reg 1 MTU ]
			&expr.Immediate{
				Register: 1,
				Data:     binaryutil.BigEndian.PutUint16(mss),
			},
			// [ exthdr write tcpopt reg 1 => 2b @ 2 + 2 ]
			&expr.Exthdr{
				SourceRegister: 1,
				Type:           2,
				Offset:         2,
				Len:            2,
				Op:             expr.ExthdrOpTcpopt,
			},
		}
	}

	if a.NfTablesRule.ClampMSS != nil {
		rule4 = append(rule4, clampMSS(nftables.TableFamilyIPv4, a.NfTablesRule.ClampMSS.MTU)...)
		rule6 = append(rule6, clampMSS(nftables.TableFamilyIPv6, a.NfTablesRule.ClampMSS.MTU)...)
	}

	if a.NfTablesRule.SetMark != nil {
		set := a.NfTablesRule.SetMark

		rulePost = append(rulePost,
			// Load the current packet mark into register 1
			&expr.Meta{
				Key:      expr.MetaKeyMARK,
				Register: 1,
			},
			// Calculate the new mark value in register 1
			&expr.Bitwise{
				SourceRegister: 1,
				DestRegister:   1,
				Len:            4,
				Xor:            binaryutil.NativeEndian.PutUint32(set.Xor),
				Mask:           binaryutil.NativeEndian.PutUint32(set.Mask),
			},
			// Set firewall mark to the value computed in register 1
			&expr.Meta{
				Key:            expr.MetaKeyMARK,
				SourceRegister: true,
				Register:       1,
			},
		)
	}

	if a.NfTablesRule.AnonCounter {
		rulePost = append(rulePost,
			// [ counter ]
			&expr.Counter{},
		)
	}

	if a.NfTablesRule.Verdict != nil {
		rulePost = append(rulePost,
			// [ verdict accept|drop ]
			&expr.Verdict{
				Kind: expr.VerdictKind(*a.NfTablesRule.Verdict),
			},
		)
	}

	// Build v4/v6 rules as requested.
	//
	// If there's no IPv4/IPv6 part, generate a single rule.
	// If there's a specific IPv4/IPv6 part, generate a rule per IP version.
	switch {
	case rule4 == nil && rule6 == nil && rulePre == nil && rulePost == nil:
		// nothing
	case rule4 == nil && rule6 == nil:
		result.Rules = [][]expr.Any{append(rulePre, rulePost...)}
	case rule4 != nil && rule6 == nil:
		result.Rules = [][]expr.Any{
			append(rulePre,
				append(
					append(matchV4, rule4...),
					rulePost...,
				)...,
			),
		}
	case rule4 == nil && rule6 != nil:
		result.Rules = [][]expr.Any{
			append(rulePre,
				append(
					append(matchV6, rule6...),
					rulePost...,
				)...,
			),
		}
	case rule4 != nil && rule6 != nil:
		result.Rules = [][]expr.Any{
			append(slices.Clone(rulePre),
				append(
					append(matchV4, rule4...),
					rulePost...,
				)...,
			),
			append(slices.Clone(rulePre),
				append(
					append(matchV6, rule6...),
					rulePost...,
				)...,
			),
		}
	}

	return &result, nil
}

func ifname(name string) []byte {
	b := make([]byte, 16)
	copy(b, []byte(name))

	return b
}
