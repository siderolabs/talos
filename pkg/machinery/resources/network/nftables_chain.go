// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// NfTablesChainType is type of NfTablesChain resource.
const NfTablesChainType = resource.Type("NfTablesChains.net.talos.dev")

// NfTablesChain resource holds definition of the nftables chain.
type NfTablesChain = typed.Resource[NfTablesChainSpec, NfTablesChainExtension]

// NfTablesChainSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type NfTablesChainSpec struct {
	Type     nethelpers.NfTablesChainType     `yaml:"type" protobuf:"1"`
	Hook     nethelpers.NfTablesChainHook     `yaml:"hook" protobuf:"2"`
	Priority nethelpers.NfTablesChainPriority `yaml:"priority" protobuf:"3"`
	Policy   nethelpers.NfTablesVerdict       `yaml:"policy" protobuf:"5"`

	Rules []NfTablesRule `yaml:"rules" protobuf:"4"`
}

// NfTablesRule describes a single rule in the nftables chain.
//
//gotagsrewrite:gen
type NfTablesRule struct {
	MatchIIfName            *NfTablesIfNameMatch         `yaml:"matchIIfName,omitempty" protobuf:"8"`
	MatchOIfName            *NfTablesIfNameMatch         `yaml:"matchOIfName,omitempty" protobuf:"1"`
	MatchMark               *NfTablesMark                `yaml:"matchMark,omitempty" protobuf:"3"`
	MatchConntrackState     *NfTablesConntrackStateMatch `yaml:"matchConntrackState,omitempty" protobuf:"11"`
	MatchSourceAddress      *NfTablesAddressMatch        `yaml:"matchSourceAddress,omitempty" protobuf:"5"`
	MatchDestinationAddress *NfTablesAddressMatch        `yaml:"matchDestinationAddress,omitempty" protobuf:"6"`
	MatchLayer4             *NfTablesLayer4Match         `yaml:"matchLayer4,omitempty" protobuf:"7"`
	MatchLimit              *NfTablesLimitMatch          `yaml:"matchLimit,omitempty" protobuf:"10"`

	ClampMSS    *NfTablesClampMSS           `yaml:"clampMSS,omitempty" protobuf:"9"`
	SetMark     *NfTablesMark               `yaml:"setMark,omitempty" protobuf:"4"`
	AnonCounter bool                        `yaml:"anonymousCounter,omitempty" protobuf:"12"`
	Verdict     *nethelpers.NfTablesVerdict `yaml:"verdict,omitempty" protobuf:"2"`
}

// NfTablesIfNameMatch describes the match on the interface name.
//
//gotagsrewrite:gen
type NfTablesIfNameMatch struct {
	InterfaceNames []string                 `yaml:"interfaceName" protobuf:"3"`
	Operator       nethelpers.MatchOperator `yaml:"operator" protobuf:"2"`
}

// NfTablesMark encodes packet mark match/update operation.
//
// When used as a match computes the following condition:
// (mark & mask) ^ xor == value
//
// When used as an update computes the following operation:
// mark = (mark & mask) ^ xor.
//
//gotagsrewrite:gen
type NfTablesMark struct {
	Mask  uint32 `yaml:"mask,omitempty" protobuf:"1"`
	Xor   uint32 `yaml:"xor,omitempty" protobuf:"2"`
	Value uint32 `yaml:"value,omitempty" protobuf:"3"`
}

// NfTablesAddressMatch describes the match on the IP address.
//
//gotagsrewrite:gen
type NfTablesAddressMatch struct {
	IncludeSubnets []netip.Prefix `yaml:"includeSubnets,omitempty" protobuf:"1"`
	ExcludeSubnets []netip.Prefix `yaml:"excludeSubnets,omitempty" protobuf:"2"`
	Invert         bool           `yaml:"invert,omitempty" protobuf:"3"`
}

// NfTablesLayer4Match describes the match on the transport layer protocol.
//
//gotagsrewrite:gen
type NfTablesLayer4Match struct {
	Protocol             nethelpers.Protocol    `yaml:"protocol" protobuf:"1"`
	MatchSourcePort      *NfTablesPortMatch     `yaml:"matchSourcePort,omitempty" protobuf:"2"`
	MatchDestinationPort *NfTablesPortMatch     `yaml:"matchDestinationPort,omitempty" protobuf:"3"`
	MatchICMPType        *NfTablesICMPTypeMatch `yaml:"matchICMPType,omitempty" protobuf:"4"`
}

// NfTablesPortMatch describes the match on the transport layer port.
//
//gotagsrewrite:gen
type NfTablesPortMatch struct {
	Ranges []PortRange `yaml:"ranges,omitempty" protobuf:"1"`
}

// NfTablesICMPTypeMatch describes the match on the ICMP type.
//
//gotagsrewrite:gen
type NfTablesICMPTypeMatch struct {
	Types []nethelpers.ICMPType `yaml:"types" protobuf:"1"`
}

// PortRange describes a range of ports.
//
// Range is [lo, hi].
//
//gotagsrewrite:gen
type PortRange struct {
	Lo uint16 `yaml:"lo" protobuf:"1"`
	Hi uint16 `yaml:"hi" protobuf:"2"`
}

// NfTablesClampMSS describes the TCP MSS clamping operation.
//
// MSS is limited by the `MaxMTU` so that:
// - IPv4: MSS = MaxMTU - 40
// - IPv6: MSS = MaxMTU - 60.
//
//gotagsrewrite:gen
type NfTablesClampMSS struct {
	MTU uint16 `yaml:"mtu" protobuf:"1"`
}

// NfTablesLimitMatch describes the match on the packet rate.
//
//gotagsrewrite:gen
type NfTablesLimitMatch struct {
	PacketRatePerSecond uint64 `yaml:"packetRatePerSecond" protobuf:"1"`
}

// NfTablesConntrackStateMatch describes the match on the connection tracking state.
//
//gotagsrewrite:gen
type NfTablesConntrackStateMatch struct {
	States []nethelpers.ConntrackState `yaml:"states" protobuf:"1"`
}

// NewNfTablesChain initializes a NfTablesChain resource.
func NewNfTablesChain(namespace resource.Namespace, id resource.ID) *NfTablesChain {
	return typed.NewResource[NfTablesChainSpec, NfTablesChainExtension](
		resource.NewMetadata(namespace, NfTablesChainType, id, resource.VersionUndefined),
		NfTablesChainSpec{},
	)
}

// NfTablesChainExtension provides auxiliary methods for NfTablesChain.
type NfTablesChainExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NfTablesChainExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NfTablesChainType,
		Aliases:          []resource.Type{"chain", "chains"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Type",
				JSONPath: `{.type}`,
			},
			{
				Name:     "Hook",
				JSONPath: `{.hook}`,
			},
			{
				Name:     "Priority",
				JSONPath: `{.priority}`,
			},
			{
				Name:     "Policy",
				JSONPath: `{.policy}`,
			},
		},
		Sensitivity: meta.NonSensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NfTablesChainSpec](NfTablesChainType, &NfTablesChain{})
	if err != nil {
		panic(err)
	}
}
