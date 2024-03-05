// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"cmp"
	"net/netip"
	"slices"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// VLANSpec describes VLAN settings if Kind == "vlan".
//
//gotagsrewrite:gen
type VLANSpec struct {
	// VID is the vlan ID.
	VID uint16 `yaml:"vlanID" protobuf:"1"`

	// Protocol is the vlan protocol.
	Protocol nethelpers.VLANProtocol `yaml:"vlanProtocol" protobuf:"2"`
}

// BondMasterSpec describes bond settings if Kind == "bond".
//
//gotagsrewrite:gen
type BondMasterSpec struct {
	Mode            nethelpers.BondMode           `yaml:"mode" protobuf:"1"`
	HashPolicy      nethelpers.BondXmitHashPolicy `yaml:"xmitHashPolicy" protobuf:"2"`
	LACPRate        nethelpers.LACPRate           `yaml:"lacpRate" protobuf:"3"`
	ARPValidate     nethelpers.ARPValidate        `yaml:"arpValidate" protobuf:"4"`
	ARPAllTargets   nethelpers.ARPAllTargets      `yaml:"arpAllTargets" protobuf:"5"`
	PrimaryIndex    uint32                        `yaml:"primary,omitempty" protobuf:"6"`
	PrimaryReselect nethelpers.PrimaryReselect    `yaml:"primaryReselect" protobuf:"7"`
	FailOverMac     nethelpers.FailOverMAC        `yaml:"failOverMac" protobuf:"8"`
	ADSelect        nethelpers.ADSelect           `yaml:"adSelect,omitempty" protobuf:"9"`
	MIIMon          uint32                        `yaml:"miimon,omitempty" protobuf:"10"`
	UpDelay         uint32                        `yaml:"updelay,omitempty" protobuf:"11"`
	DownDelay       uint32                        `yaml:"downdelay,omitempty" protobuf:"12"`
	ARPInterval     uint32                        `yaml:"arpInterval,omitempty" protobuf:"13"`
	ResendIGMP      uint32                        `yaml:"resendIgmp,omitempty" protobuf:"14"`
	MinLinks        uint32                        `yaml:"minLinks,omitempty" protobuf:"15"`
	LPInterval      uint32                        `yaml:"lpInterval,omitempty" protobuf:"16"`
	PacketsPerSlave uint32                        `yaml:"packetsPerSlave,omitempty" protobuf:"17"`
	NumPeerNotif    uint8                         `yaml:"numPeerNotif,omitempty" protobuf:"18"`
	TLBDynamicLB    uint8                         `yaml:"tlbLogicalLb,omitempty" protobuf:"19"`
	AllSlavesActive uint8                         `yaml:"allSlavesActive,omitempty" protobuf:"20"`
	UseCarrier      bool                          `yaml:"useCarrier,omitempty" protobuf:"21"`
	ADActorSysPrio  uint16                        `yaml:"adActorSysPrio,omitempty" protobuf:"22"`
	ADUserPortKey   uint16                        `yaml:"adUserPortKey,omitempty" protobuf:"23"`
	PeerNotifyDelay uint32                        `yaml:"peerNotifyDelay,omitempty" protobuf:"24"`
}

// BridgeMasterSpec describes bridge settings if Kind == "bridge".
//
//gotagsrewrite:gen
type BridgeMasterSpec struct {
	STP STPSpec `yaml:"stp,omitempty" protobuf:"1"`
}

// STPSpec describes Spanning Tree Protocol (STP) settings of a bridge.
//
//gotagsrewrite:gen
type STPSpec struct {
	Enabled bool `yaml:"enabled" protobuf:"1"`
}

// WireguardSpec describes Wireguard settings if Kind == "wireguard".
//
//gotagsrewrite:gen
type WireguardSpec struct {
	// PrivateKey is used to configure the link, present only in the LinkSpec.
	PrivateKey string `yaml:"privateKey,omitempty" protobuf:"1"`
	// PublicKey is only used in LinkStatus to show the link status.
	PublicKey    string          `yaml:"publicKey,omitempty" protobuf:"2"`
	ListenPort   int             `yaml:"listenPort" protobuf:"3"`
	FirewallMark int             `yaml:"firewallMark" protobuf:"4"`
	Peers        []WireguardPeer `yaml:"peers" protobuf:"5"`
}

// WireguardPeer describes a single peer.
//
//gotagsrewrite:gen
type WireguardPeer struct {
	PublicKey                   string         `yaml:"publicKey" protobuf:"1"`
	PresharedKey                string         `yaml:"presharedKey" protobuf:"2"`
	Endpoint                    string         `yaml:"endpoint" protobuf:"3"`
	PersistentKeepaliveInterval time.Duration  `yaml:"persistentKeepaliveInterval" protobuf:"4"`
	AllowedIPs                  []netip.Prefix `yaml:"allowedIPs" protobuf:"5"`
}

// ID Returns the VID for type VLANSpec.
func (vlan VLANSpec) ID() uint16 {
	return vlan.VID
}

// MTU Returns MTU=0 for type VLANSpec.
func (vlan VLANSpec) MTU() uint32 {
	return 0
}

// Equal checks two WireguardPeer structs for equality.
//
// `spec` is considered to be the result of getting current Wireguard configuration,
// while `other` is the new (updated configuration).
func (peer *WireguardPeer) Equal(other *WireguardPeer) bool {
	if peer.PublicKey != other.PublicKey {
		return false
	}

	if peer.PresharedKey != other.PresharedKey {
		return false
	}

	// if the Endpoint is not set in `other`, don't consider this to be a change
	if other.Endpoint != "" && peer.Endpoint != other.Endpoint {
		return false
	}

	if peer.PersistentKeepaliveInterval != other.PersistentKeepaliveInterval {
		return false
	}

	if len(peer.AllowedIPs) != len(other.AllowedIPs) {
		return false
	}

	for i := range peer.AllowedIPs {
		if peer.AllowedIPs[i].Addr().Compare(other.AllowedIPs[i].Addr()) != 0 {
			return false
		}

		if peer.AllowedIPs[i].Bits() != other.AllowedIPs[i].Bits() {
			return false
		}
	}

	return true
}

// IsZero checks if the WireguardSpec is zero value.
func (spec *WireguardSpec) IsZero() bool {
	return spec.PrivateKey == "" && spec.ListenPort == 0 && spec.FirewallMark == 0 && len(spec.Peers) == 0
}

// Equal checks two WireguardSpecs for equality.
//
// Both specs should be sorted before calling this method.
//
// `spec` is considered to be the result of getting current Wireguard configuration,
// while `other` is the new (updated configuration).
func (spec *WireguardSpec) Equal(other *WireguardSpec) bool {
	if spec.PrivateKey != other.PrivateKey {
		return false
	}

	// listenPort of '0' means use any available port, so we shouldn't consider this to be a "change"
	if spec.ListenPort != other.ListenPort && other.ListenPort != 0 {
		return false
	}

	if spec.FirewallMark != other.FirewallMark {
		return false
	}

	if len(spec.Peers) != len(other.Peers) {
		return false
	}

	for i := range spec.Peers {
		if !spec.Peers[i].Equal(&other.Peers[i]) {
			return false
		}
	}

	return true
}

// Sort the spec so that comparison is possible.
func (spec *WireguardSpec) Sort() {
	slices.SortFunc(spec.Peers, func(a, b WireguardPeer) int { return cmp.Compare(a.PublicKey, b.PublicKey) })

	for k := range spec.Peers {
		slices.SortFunc(spec.Peers[k].AllowedIPs, func(left, right netip.Prefix) int {
			if res := left.Addr().Compare(right.Addr()); res != 0 {
				return res
			}

			return cmp.Compare(left.Bits(), right.Bits())
		})
	}
}

// Merge with other Wireguard spec overwriting non-zero values.
func (spec *WireguardSpec) Merge(other WireguardSpec) {
	if other.ListenPort != 0 {
		spec.ListenPort = other.ListenPort
	}

	if other.FirewallMark != 0 {
		spec.FirewallMark = other.FirewallMark
	}

	if other.PrivateKey != "" {
		spec.PrivateKey = other.PrivateKey
	}

	// avoid adding same peer twice, no real peer information merging for now
	for _, peer := range other.Peers {
		exists := false

		for _, p := range spec.Peers {
			if p.PublicKey == peer.PublicKey {
				exists = true

				break
			}
		}

		if !exists {
			spec.Peers = append(spec.Peers, peer)
		}
	}
}
