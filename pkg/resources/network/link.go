// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package network

import (
	"encoding/binary"
	"net"
	"sort"
	"time"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// VLANSpec describes VLAN settings if Kind == "vlan".
type VLANSpec struct {
	// VID is the vlan ID.
	VID uint16 `yaml:"vlanID"`

	// Protocol is the vlan protocol.
	Protocol nethelpers.VLANProtocol `yaml:"vlanProtocol"`
}

// Encode the VLANSpec into netlink attributes.
func (vlan *VLANSpec) Encode() ([]byte, error) {
	encoder := netlink.NewAttributeEncoder()

	encoder.Uint16(unix.IFLA_VLAN_ID, vlan.VID)

	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(vlan.Protocol))
	encoder.Bytes(unix.IFLA_VLAN_PROTOCOL, buf)

	return encoder.Encode()
}

// Decode the VLANSpec from netlink attributes.
func (vlan *VLANSpec) Decode(data []byte) error {
	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		switch decoder.Type() {
		case unix.IFLA_VLAN_ID:
			vlan.VID = decoder.Uint16()
		case unix.IFLA_VLAN_PROTOCOL:
			vlan.Protocol = nethelpers.VLANProtocol(binary.BigEndian.Uint16(decoder.Bytes()))
		}
	}

	return decoder.Err()
}

// BondMasterSpec describes bond settings if Kind == "bond".
type BondMasterSpec struct {
	Mode            nethelpers.BondMode           `yaml:"mode"`
	HashPolicy      nethelpers.BondXmitHashPolicy `yaml:"xmitHashPolicy"`
	LACPRate        nethelpers.LACPRate           `yaml:"lacpRate"`
	ARPValidate     nethelpers.ARPValidate        `yaml:"arpValidate"`
	ARPAllTargets   nethelpers.ARPAllTargets      `yaml:"arpAllTargets"`
	PrimaryIndex    uint32                        `yaml:"primary,omitempty"`
	PrimaryReselect nethelpers.PrimaryReselect    `yaml:"primaryReselect"`
	FailOverMac     nethelpers.FailOverMAC        `yaml:"failOverMac"`
	ADSelect        nethelpers.ADSelect           `yaml:"adSelect,omitempty"`
	MIIMon          uint32                        `yaml:"miimon,omitempty"`
	UpDelay         uint32                        `yaml:"updelay,omitempty"`
	DownDelay       uint32                        `yaml:"downdelay,omitempty"`
	ARPInterval     uint32                        `yaml:"arpInterval,omitempty"`
	ResendIGMP      uint32                        `yaml:"resendIgmp,omitempty"`
	MinLinks        uint32                        `yaml:"minLinks,omitempty"`
	LPInterval      uint32                        `yaml:"lpInterval,omitempty"`
	PacketsPerSlave uint32                        `yaml:"packetsPerSlave,omitempty"`
	NumPeerNotif    uint8                         `yaml:"numPeerNotif,omitempty"`
	TLBDynamicLB    uint8                         `yaml:"tlbLogicalLb,omitempty"`
	AllSlavesActive uint8                         `yaml:"allSlavesActive,omitempty"`
	UseCarrier      bool                          `yaml:"useCarrier,omitempty"`
	ADActorSysPrio  uint16                        `yaml:"adActorSysPrio,omitempty"`
	ADUserPortKey   uint16                        `yaml:"adUserPortKey,omitempty"`
	PeerNotifyDelay uint32                        `yaml:"peerNotifyDelay,omitempty"`
}

// FillDefaults fills zero values with proper default values.
func (bond *BondMasterSpec) FillDefaults() {
	if bond.ResendIGMP == 0 {
		bond.ResendIGMP = 1
	}

	if bond.LPInterval == 0 {
		bond.LPInterval = 1
	}

	if bond.PacketsPerSlave == 0 {
		bond.PacketsPerSlave = 1
	}

	if bond.NumPeerNotif == 0 {
		bond.NumPeerNotif = 1
	}

	if bond.Mode != nethelpers.BondModeALB && bond.Mode != nethelpers.BondModeTLB {
		bond.TLBDynamicLB = 1
	}

	if bond.Mode == nethelpers.BondMode8023AD {
		bond.ADActorSysPrio = 65535
	}
}

// Encode the BondMasterSpec into netlink attributes.
//
//nolint:gocyclo
func (bond *BondMasterSpec) Encode() ([]byte, error) {
	encoder := netlink.NewAttributeEncoder()

	encoder.Uint8(unix.IFLA_BOND_MODE, uint8(bond.Mode))
	encoder.Uint8(unix.IFLA_BOND_XMIT_HASH_POLICY, uint8(bond.HashPolicy))

	if bond.Mode == nethelpers.BondMode8023AD {
		encoder.Uint8(unix.IFLA_BOND_AD_LACP_RATE, uint8(bond.LACPRate))
	}

	if bond.Mode != nethelpers.BondMode8023AD && bond.Mode != nethelpers.BondModeALB && bond.Mode != nethelpers.BondModeTLB {
		encoder.Uint32(unix.IFLA_BOND_ARP_VALIDATE, uint32(bond.ARPValidate))
	}

	encoder.Uint32(unix.IFLA_BOND_ARP_ALL_TARGETS, uint32(bond.ARPAllTargets))

	if bond.Mode == nethelpers.BondModeActiveBackup || bond.Mode == nethelpers.BondModeALB || bond.Mode == nethelpers.BondModeTLB {
		encoder.Uint32(unix.IFLA_BOND_PRIMARY, bond.PrimaryIndex)
	}

	encoder.Uint8(unix.IFLA_BOND_PRIMARY_RESELECT, uint8(bond.PrimaryReselect))
	encoder.Uint8(unix.IFLA_BOND_FAIL_OVER_MAC, uint8(bond.FailOverMac))
	encoder.Uint8(unix.IFLA_BOND_AD_SELECT, uint8(bond.ADSelect))
	encoder.Uint32(unix.IFLA_BOND_MIIMON, bond.MIIMon)

	if bond.MIIMon != 0 {
		encoder.Uint32(unix.IFLA_BOND_UPDELAY, bond.UpDelay)
		encoder.Uint32(unix.IFLA_BOND_DOWNDELAY, bond.DownDelay)
	}

	if bond.Mode != nethelpers.BondMode8023AD && bond.Mode != nethelpers.BondModeALB && bond.Mode != nethelpers.BondModeTLB {
		encoder.Uint32(unix.IFLA_BOND_ARP_INTERVAL, bond.ARPInterval)
	}

	encoder.Uint32(unix.IFLA_BOND_RESEND_IGMP, bond.ResendIGMP)
	encoder.Uint32(unix.IFLA_BOND_MIN_LINKS, bond.MinLinks)
	encoder.Uint32(unix.IFLA_BOND_LP_INTERVAL, bond.LPInterval)

	if bond.Mode == nethelpers.BondModeRoundrobin {
		encoder.Uint32(unix.IFLA_BOND_PACKETS_PER_SLAVE, bond.PacketsPerSlave)
	}

	encoder.Uint8(unix.IFLA_BOND_NUM_PEER_NOTIF, bond.NumPeerNotif)

	if bond.Mode == nethelpers.BondModeALB || bond.Mode == nethelpers.BondModeTLB {
		encoder.Uint8(unix.IFLA_BOND_TLB_DYNAMIC_LB, bond.TLBDynamicLB)
	}

	encoder.Uint8(unix.IFLA_BOND_ALL_SLAVES_ACTIVE, bond.AllSlavesActive)

	var useCarrier uint8

	if bond.UseCarrier {
		useCarrier = 1
	}

	encoder.Uint8(unix.IFLA_BOND_USE_CARRIER, useCarrier)

	if bond.Mode == nethelpers.BondMode8023AD {
		encoder.Uint16(unix.IFLA_BOND_AD_ACTOR_SYS_PRIO, bond.ADActorSysPrio)
		encoder.Uint16(unix.IFLA_BOND_AD_USER_PORT_KEY, bond.ADUserPortKey)
	}

	if bond.MIIMon != 0 {
		encoder.Uint32(unix.IFLA_BOND_PEER_NOTIF_DELAY, bond.PeerNotifyDelay)
	}

	return encoder.Encode()
}

// Decode the BondMasterSpec from netlink attributes.
//
//nolint:gocyclo,cyclop
func (bond *BondMasterSpec) Decode(data []byte) error {
	decoder, err := netlink.NewAttributeDecoder(data)
	if err != nil {
		return err
	}

	for decoder.Next() {
		switch decoder.Type() {
		case unix.IFLA_BOND_MODE:
			bond.Mode = nethelpers.BondMode(decoder.Uint8())
		case unix.IFLA_BOND_XMIT_HASH_POLICY:
			bond.HashPolicy = nethelpers.BondXmitHashPolicy(decoder.Uint8())
		case unix.IFLA_BOND_AD_LACP_RATE:
			bond.LACPRate = nethelpers.LACPRate(decoder.Uint8())
		case unix.IFLA_BOND_ARP_VALIDATE:
			bond.ARPValidate = nethelpers.ARPValidate(decoder.Uint32())
		case unix.IFLA_BOND_ARP_ALL_TARGETS:
			bond.ARPAllTargets = nethelpers.ARPAllTargets(decoder.Uint32())
		case unix.IFLA_BOND_PRIMARY:
			bond.PrimaryIndex = decoder.Uint32()
		case unix.IFLA_BOND_PRIMARY_RESELECT:
			bond.PrimaryReselect = nethelpers.PrimaryReselect(decoder.Uint8())
		case unix.IFLA_BOND_FAIL_OVER_MAC:
			bond.FailOverMac = nethelpers.FailOverMAC(decoder.Uint8())
		case unix.IFLA_BOND_AD_SELECT:
			bond.ADSelect = nethelpers.ADSelect(decoder.Uint8())
		case unix.IFLA_BOND_MIIMON:
			bond.MIIMon = decoder.Uint32()
		case unix.IFLA_BOND_UPDELAY:
			bond.UpDelay = decoder.Uint32()
		case unix.IFLA_BOND_DOWNDELAY:
			bond.DownDelay = decoder.Uint32()
		case unix.IFLA_BOND_ARP_INTERVAL:
			bond.ARPInterval = decoder.Uint32()
		case unix.IFLA_BOND_RESEND_IGMP:
			bond.ResendIGMP = decoder.Uint32()
		case unix.IFLA_BOND_MIN_LINKS:
			bond.MinLinks = decoder.Uint32()
		case unix.IFLA_BOND_LP_INTERVAL:
			bond.LPInterval = decoder.Uint32()
		case unix.IFLA_BOND_PACKETS_PER_SLAVE:
			bond.PacketsPerSlave = decoder.Uint32()
		case unix.IFLA_BOND_NUM_PEER_NOTIF:
			bond.NumPeerNotif = decoder.Uint8()
		case unix.IFLA_BOND_TLB_DYNAMIC_LB:
			bond.TLBDynamicLB = decoder.Uint8()
		case unix.IFLA_BOND_ALL_SLAVES_ACTIVE:
			bond.AllSlavesActive = decoder.Uint8()
		case unix.IFLA_BOND_USE_CARRIER:
			bond.UseCarrier = decoder.Uint8() == 1
		case unix.IFLA_BOND_AD_ACTOR_SYS_PRIO:
			bond.ADActorSysPrio = decoder.Uint16()
		case unix.IFLA_BOND_AD_USER_PORT_KEY:
			bond.ADUserPortKey = decoder.Uint16()
		case unix.IFLA_BOND_PEER_NOTIF_DELAY:
			bond.PeerNotifyDelay = decoder.Uint32()
		}
	}

	return decoder.Err()
}

// WireguardSpec describes Wireguard settings if Kind == "wireguard".
type WireguardSpec struct {
	// PrivateKey is used to configure the link, present only in the LinkSpec.
	PrivateKey string `yaml:"privateKey,omitempty"`
	// PublicKey is only used in LinkStatus to show the link status.
	PublicKey    string          `yaml:"publicKey,omitempty"`
	ListenPort   int             `yaml:"listenPort"`
	FirewallMark int             `yaml:"firewallMark"`
	Peers        []WireguardPeer `yaml:"peers"`
}

// WireguardPeer describes a single peer.
type WireguardPeer struct {
	PublicKey                   string             `yaml:"publicKey"`
	PresharedKey                string             `yaml:"presharedKey"`
	Endpoint                    string             `yaml:"endpoint"`
	PersistentKeepaliveInterval time.Duration      `yaml:"persistentKeepaliveInterval"`
	AllowedIPs                  []netaddr.IPPrefix `yaml:"allowedIPs"`
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
		if peer.AllowedIPs[i].IP().Compare(other.AllowedIPs[i].IP()) != 0 {
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
	sort.Slice(spec.Peers, func(i, j int) bool {
		return spec.Peers[i].PublicKey < spec.Peers[j].PublicKey
	})

	for k := range spec.Peers {
		k := k

		sort.Slice(spec.Peers[k].AllowedIPs, func(i, j int) bool {
			left := spec.Peers[k].AllowedIPs[i]
			right := spec.Peers[k].AllowedIPs[j]

			switch left.IP().Compare(right.IP()) {
			case -1:
				return true
			case 0:
				return left.Bits() < right.Bits()
			default:
				return false
			}
		})
	}
}

// Encode converts WireguardSpec to wgctrl.Config "patch" to adjust the config to match the spec.
//
// Both specs should be sorted.
//
// Encode produces a "diff" as *wgtypes.Config which when applied transitions `existing` configuration into
// configuration `spec`.
//
//nolint:gocyclo,cyclop
func (spec *WireguardSpec) Encode(existing *WireguardSpec) (*wgtypes.Config, error) {
	cfg := &wgtypes.Config{}

	if existing.PrivateKey != spec.PrivateKey {
		key, err := wgtypes.ParseKey(spec.PrivateKey)
		if err != nil {
			return nil, err
		}

		cfg.PrivateKey = &key
	}

	if existing.ListenPort != spec.ListenPort {
		cfg.ListenPort = &spec.ListenPort
	}

	if existing.FirewallMark != spec.FirewallMark {
		cfg.FirewallMark = &spec.FirewallMark
	}

	// perform a merge of two sorted list of peers producing diff
	l, r := 0, 0

	for l < len(existing.Peers) || r < len(spec.Peers) {
		addPeer := func(peer *WireguardPeer) error {
			pubKey, err := wgtypes.ParseKey(peer.PublicKey)
			if err != nil {
				return err
			}

			var presharedKey *wgtypes.Key

			if peer.PresharedKey != "" {
				var parsedKey wgtypes.Key

				parsedKey, err = wgtypes.ParseKey(peer.PresharedKey)
				if err != nil {
					return err
				}

				presharedKey = &parsedKey
			}

			var endpoint *net.UDPAddr

			if peer.Endpoint != "" {
				endpoint, err = net.ResolveUDPAddr("", peer.Endpoint)
				if err != nil {
					return err
				}
			}

			allowedIPs := make([]net.IPNet, len(peer.AllowedIPs))

			for i := range peer.AllowedIPs {
				allowedIPs[i] = *peer.AllowedIPs[i].IPNet()
			}

			cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
				PublicKey:                   pubKey,
				Endpoint:                    endpoint,
				PresharedKey:                presharedKey,
				PersistentKeepaliveInterval: &peer.PersistentKeepaliveInterval,
				ReplaceAllowedIPs:           true,
				AllowedIPs:                  allowedIPs,
			})

			return nil
		}

		deletePeer := func(peer *WireguardPeer) error {
			pubKey, err := wgtypes.ParseKey(peer.PublicKey)
			if err != nil {
				return err
			}

			cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
				PublicKey: pubKey,
				Remove:    true,
			})

			return nil
		}

		var left, right *WireguardPeer

		if l < len(existing.Peers) {
			left = &existing.Peers[l]
		}

		if r < len(spec.Peers) {
			right = &spec.Peers[r]
		}

		switch {
		// peer from the "right" (new spec) is missing in "existing" (left), add it
		case left == nil || (right != nil && left.PublicKey > right.PublicKey):
			if err := addPeer(right); err != nil {
				return nil, err
			}

			r++
		// peer from the "left" (existing) is missing in new spec (right), so it should be removed
		case right == nil || (left != nil && left.PublicKey < right.PublicKey):
			// deleting peers from the existing
			if err := deletePeer(left); err != nil {
				return nil, err
			}

			l++
		// peer public keys are equal, so either they are identical or peer should be replaced
		case left.PublicKey == right.PublicKey:
			if !left.Equal(right) {
				// replace peer
				if err := addPeer(right); err != nil {
					return nil, err
				}
			}

			l++
			r++
		}
	}

	return cfg, nil
}

// Decode spec from the device state.
func (spec *WireguardSpec) Decode(dev *wgtypes.Device, isStatus bool) {
	if isStatus {
		spec.PublicKey = dev.PublicKey.String()
	} else {
		spec.PrivateKey = dev.PrivateKey.String()
	}

	spec.ListenPort = dev.ListenPort
	spec.FirewallMark = dev.FirewallMark

	spec.Peers = make([]WireguardPeer, len(dev.Peers))

	for i := range spec.Peers {
		spec.Peers[i].PublicKey = dev.Peers[i].PublicKey.String()

		if dev.Peers[i].Endpoint != nil {
			spec.Peers[i].Endpoint = dev.Peers[i].Endpoint.String()
		}

		var zeroKey wgtypes.Key

		if dev.Peers[i].PresharedKey != zeroKey {
			spec.Peers[i].PresharedKey = dev.Peers[i].PresharedKey.String()
		}

		spec.Peers[i].PersistentKeepaliveInterval = dev.Peers[i].PersistentKeepaliveInterval
		spec.Peers[i].AllowedIPs = make([]netaddr.IPPrefix, len(dev.Peers[i].AllowedIPs))

		for j := range dev.Peers[i].AllowedIPs {
			spec.Peers[i].AllowedIPs[j], _ = netaddr.FromStdIPNet(&dev.Peers[i].AllowedIPs[j])
		}
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
