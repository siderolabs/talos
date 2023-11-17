// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"os"
	"slices"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/siderolabs/gen/xslices"
	"go4.org/netipx"
	"golang.org/x/sys/unix"

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
)

// NfTablesSet is a compiled representation of the set.
type NfTablesSet struct {
	Kind      SetKind
	Addresses []netipx.IPRange
	Ports     [][2]uint16
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
		elements := make([]nftables.SetElement, 0, len(set.Ports))

		for _, p := range set.Ports {
			from := binaryutil.BigEndian.PutUint16(p[0])
			to := binaryutil.BigEndian.PutUint16(p[1])

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
	default:
		panic(fmt.Sprintf("unknown set kind: %d", set.Kind))
	}
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

	if a.NfTablesRule.MatchIIfName != nil {
		match := a.NfTablesRule.MatchIIfName

		rulePre = append(rulePre,
			// [ meta load iifname => reg 1 ]
			&expr.Meta{
				Key:      expr.MetaKeyIIFNAME,
				Register: 1,
			},
			// [ cmp eq/neq reg 1 <ifname> ]
			&expr.Cmp{
				Op:       expr.CmpOp(match.Operator),
				Register: 1,
				Data:     ifname(match.InterfaceName),
			},
		)
	}

	if a.NfTablesRule.MatchOIfName != nil {
		match := a.NfTablesRule.MatchOIfName

		rulePre = append(rulePre,
			// [ meta load oifname => reg 1 ]
			&expr.Meta{
				Key:      expr.MetaKeyOIFNAME,
				Register: 1,
			},
			// [ cmp eq/neq reg 1 <ifname> ]
			&expr.Cmp{
				Op:       expr.CmpOp(match.Operator),
				Register: 1,
				Data:     ifname(match.InterfaceName),
			},
		)
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

		// skip v4 rule if there are not IPs to match for and not inverted
		if v4Set != nil || match.Invert {
			if v4Set == nil && match.Invert {
				// match any v4 IP
				if rule4 == nil {
					rule4 = []expr.Any{}
				}
			} else {
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
			}
		}

		// skip v6 rule if there are not IPs to match for and not inverted
		if v6Set != nil || match.Invert {
			if v6Set == nil && match.Invert {
				// match any v6 IP
				if rule6 == nil {
					rule6 = []expr.Any{}
				}
			} else {
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
			}
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
