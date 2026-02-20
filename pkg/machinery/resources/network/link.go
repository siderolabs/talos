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
	// Mode specifies the bonding policy
	Mode nethelpers.BondMode `yaml:"mode" protobuf:"1"`
	// HashPolicy selects the transmit hash policy to use for slave selection.
	HashPolicy nethelpers.BondXmitHashPolicy `yaml:"xmitHashPolicy" protobuf:"2"`
	// LACPRate specifies the rate at which LACPDU frames are sent.
	LACPRate nethelpers.LACPRate `yaml:"lacpRate" protobuf:"3"`
	// ARPValidate specifies whether or not ARP probes and replies should be validated.
	ARPValidate nethelpers.ARPValidate `yaml:"arpValidate" protobuf:"4"`
	// ARPAllTargets specifies whether ARP probes should be sent to any or all targets.
	ARPAllTargets nethelpers.ARPAllTargets `yaml:"arpAllTargets" protobuf:"5"`
	// PrimaryIndex is a device index specifying which slave is the primary device.
	PrimaryIndex *uint32 `yaml:"primary,omitempty" protobuf:"6"`
	// PrimaryReselect specifies the policy under which the primary slave should be reselected.
	PrimaryReselect nethelpers.PrimaryReselect `yaml:"primaryReselect" protobuf:"7"`
	// FailOverMac whether active-backup mode should set all slaves to the same MAC address at enslavement, when enabled, or perform special handling.
	FailOverMac nethelpers.FailOverMAC `yaml:"failOverMac" protobuf:"8"`
	// ADSelect specifies the aggregate selection policy for 802.3ad.
	ADSelect nethelpers.ADSelect `yaml:"adSelect,omitempty" protobuf:"9"`
	// MIIMon is the link monitoring frequency in milliseconds.
	MIIMon uint32 `yaml:"miimon,omitempty" protobuf:"10"`
	// UpDelay is the time, in milliseconds, to wait before enabling a slave after a link recovery has been detected.
	UpDelay uint32 `yaml:"updelay,omitempty" protobuf:"11"`
	// DownDelay is the time, in milliseconds, to wait before disabling a slave after a link failure has been detected.
	DownDelay uint32 `yaml:"downdelay,omitempty" protobuf:"12"`
	// ARPInterval is the ARP link monitoring frequency in milliseconds.
	ARPInterval uint32 `yaml:"arpInterval,omitempty" protobuf:"13"`
	// ResendIGMP specifies the number of times IGMP packets should be resent.
	ResendIGMP uint32 `yaml:"resendIgmp,omitempty" protobuf:"14"`
	// MinLinks specifies the minimum number of active links to assert carrier.
	MinLinks uint32 `yaml:"minLinks,omitempty" protobuf:"15"`
	// LPInterval specifies the number of seconds between instances where the bonding driver sends learning packets to each slave's peer switch.
	LPInterval uint32 `yaml:"lpInterval,omitempty" protobuf:"16"`
	// PacketsPerSlave specifies the number of packets to transmit through a slave before moving to the next one.
	PacketsPerSlave uint32 `yaml:"packetsPerSlave,omitempty" protobuf:"17"`
	// NumPeerNotif specifies the number of peer notifications
	// (gratuitous ARPs and unsolicited IPv6 Neighbor Advertisements) to be issued after a failover event.
	NumPeerNotif uint8 `yaml:"numPeerNotif,omitempty" protobuf:"18"`
	// TLBDynamicLB specifies if dynamic shuffling of flows is enabled in tlb or alb mode.
	TLBDynamicLB uint8 `yaml:"tlbLogicalLb,omitempty" protobuf:"19"`
	// AllSlavesActive specifies that duplicate frames (received on inactive ports) should be dropped (0) or delivered (1).
	AllSlavesActive uint8 `yaml:"allSlavesActive,omitempty" protobuf:"20"`
	// UseCarrier specifies whether or not miimon should use MII or ETHTOOL.
	UseCarrier bool `yaml:"useCarrier,omitempty" protobuf:"21"`
	// ADActorSysPrio is the actor system priority for 802.3ad.
	ADActorSysPrio uint16 `yaml:"adActorSysPrio,omitempty" protobuf:"22"`
	// ADUserPortKey is the user port key (upper 10 bits) for 802.3ad.
	ADUserPortKey uint16 `yaml:"adUserPortKey,omitempty" protobuf:"23"`
	// PeerNotifyDelay is the delay, in milliseconds, between each peer notification.
	PeerNotifyDelay uint32 `yaml:"peerNotifyDelay,omitempty" protobuf:"24"`
	// ARPIPTargets is the list of IP addresses to use for ARP link monitoring when ARPInterval is set.
	//
	// Maximum of 16 targets are supported.
	ARPIPTargets []netip.Addr `yaml:"arpIpTargets,omitempty" protobuf:"25"`
	// NSIP6Targets is the list of IPv6 addresses to use for NS link monitoring when ARPInterval is set.
	//
	// Maximum of 16 targets are supported.
	NSIP6Targets []netip.Addr `yaml:"nsIp6Targets,omitempty" protobuf:"26"`
	// ADLACPActive specifies whether to send LACPDU frames periodically.
	ADLACPActive nethelpers.ADLACPActive `yaml:"adLacpActive,omitempty" protobuf:"27"`
	// MissedMax is the number of arp_interval monitor checks that must fail in order for an interface to be marked down by the ARP monitor.
	MissedMax uint8 `yaml:"missedMax,omitempty" protobuf:"28"`
}

// Equal checks two BondMasterSpecs for equality.
//
//nolint:gocyclo,cyclop
func (spec *BondMasterSpec) Equal(other *BondMasterSpec) bool {
	if spec.Mode != other.Mode {
		return false
	}

	if spec.HashPolicy != other.HashPolicy {
		return false
	}

	if spec.LACPRate != other.LACPRate {
		return false
	}

	if spec.ARPValidate != other.ARPValidate {
		return false
	}

	if spec.ARPAllTargets != other.ARPAllTargets {
		return false
	}

	if spec.PrimaryIndex != nil && other.PrimaryIndex != nil && *spec.PrimaryIndex != *other.PrimaryIndex {
		return false
	}

	if spec.PrimaryReselect != other.PrimaryReselect {
		return false
	}

	if spec.FailOverMac != other.FailOverMac {
		return false
	}

	if spec.ADSelect != other.ADSelect {
		return false
	}

	if spec.MIIMon != other.MIIMon {
		return false
	}

	if spec.UpDelay != other.UpDelay {
		return false
	}

	if spec.DownDelay != other.DownDelay {
		return false
	}

	if spec.ARPInterval != other.ARPInterval {
		return false
	}

	if spec.ResendIGMP != other.ResendIGMP {
		return false
	}

	if spec.MinLinks != other.MinLinks {
		return false
	}

	if spec.LPInterval != other.LPInterval {
		return false
	}

	if spec.PacketsPerSlave != other.PacketsPerSlave {
		return false
	}

	if spec.NumPeerNotif != other.NumPeerNotif {
		return false
	}

	if spec.TLBDynamicLB != other.TLBDynamicLB {
		return false
	}

	if spec.AllSlavesActive != other.AllSlavesActive {
		return false
	}

	if spec.UseCarrier != other.UseCarrier {
		return false
	}

	if spec.ADActorSysPrio != other.ADActorSysPrio {
		return false
	}

	if spec.ADUserPortKey != other.ADUserPortKey {
		return false
	}

	if spec.PeerNotifyDelay != other.PeerNotifyDelay {
		return false
	}

	if len(spec.ARPIPTargets) != len(other.ARPIPTargets) {
		return false
	}

	for i := range spec.ARPIPTargets {
		if spec.ARPIPTargets[i] != other.ARPIPTargets[i] {
			return false
		}
	}

	if len(spec.NSIP6Targets) != len(other.NSIP6Targets) {
		return false
	}

	for i := range spec.NSIP6Targets {
		if spec.NSIP6Targets[i] != other.NSIP6Targets[i] {
			return false
		}
	}

	if spec.ADLACPActive != other.ADLACPActive {
		return false
	}

	if spec.Mode != nethelpers.BondMode8023AD && spec.Mode != nethelpers.BondModeALB && spec.Mode != nethelpers.BondModeTLB {
		if spec.MissedMax != other.MissedMax {
			return false
		}
	}

	return true
}

// IsZero checks if the BondMasterSpec is zero value.
//
//nolint:gocyclo,cyclop
func (spec *BondMasterSpec) IsZero() bool {
	return spec.Mode == 0 &&
		spec.HashPolicy == 0 &&
		spec.LACPRate == 0 &&
		spec.ARPValidate == 0 &&
		spec.ARPAllTargets == 0 &&
		spec.PrimaryIndex == nil &&
		spec.PrimaryReselect == 0 &&
		spec.FailOverMac == 0 &&
		spec.ADSelect == 0 &&
		spec.MIIMon == 0 &&
		spec.UpDelay == 0 &&
		spec.DownDelay == 0 &&
		spec.ARPInterval == 0 &&
		spec.ResendIGMP == 0 &&
		spec.MinLinks == 0 &&
		spec.LPInterval == 0 &&
		spec.PacketsPerSlave == 0 &&
		spec.NumPeerNotif == 0 &&
		spec.TLBDynamicLB == 0 &&
		spec.AllSlavesActive == 0 &&
		!spec.UseCarrier &&
		spec.ADActorSysPrio == 0 &&
		spec.ADUserPortKey == 0 &&
		spec.PeerNotifyDelay == 0 &&
		len(spec.ARPIPTargets) == 0 &&
		len(spec.NSIP6Targets) == 0 &&
		spec.ADLACPActive == 0 &&
		spec.MissedMax == 0
}

// BridgeMasterSpec describes bridge settings if Kind == "bridge".
//
//gotagsrewrite:gen
type BridgeMasterSpec struct {
	STP  STPSpec        `yaml:"stp,omitempty" protobuf:"1"`
	VLAN BridgeVLANSpec `yaml:"vlan,omitempty" protobuf:"2"`
}

// STPSpec describes Spanning Tree Protocol (STP) settings of a bridge.
//
//gotagsrewrite:gen
type STPSpec struct {
	Enabled bool `yaml:"enabled" protobuf:"1"`
}

// BridgeVLANSpec describes VLAN settings of a bridge.
//
//gotagsrewrite:gen
type BridgeVLANSpec struct {
	FilteringEnabled bool `yaml:"filteringEnabled" protobuf:"1"`
}

// VRFMasterSpec describes vrf settings if Kind == "vrf".
//
//gotagsrewrite:gen
type VRFMasterSpec struct {
	Table nethelpers.RoutingTable `yaml:"table" protobuf:"1"`
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

// Mode returns the protocol (mode) for type VLANSpec.
func (vlan VLANSpec) Mode() nethelpers.VLANProtocol {
	return vlan.Protocol
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
