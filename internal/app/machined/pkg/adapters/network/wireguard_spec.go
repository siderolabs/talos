// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net"
	"net/netip"

	"github.com/siderolabs/gen/xslices"
	"go4.org/netipx"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// WireguardSpec adapter provides encoding/decoding to netlink structures.
//
//nolint:revive,golint
func WireguardSpec(r *network.WireguardSpec) wireguardSpec {
	return wireguardSpec{
		WireguardSpec: r,
	}
}

type wireguardSpec struct {
	*network.WireguardSpec
}

// Encode converts WireguardSpec to wgctrl.Config "patch" to adjust the config to match the spec.
//
// Both specs should be sorted.
//
// Encode produces a "diff" as *wgtypes.Config which when applied transitions `existing` configuration into
// configuration `spec`.
//
//nolint:gocyclo,cyclop
func (a wireguardSpec) Encode(existing *network.WireguardSpec) (*wgtypes.Config, error) {
	spec := a.WireguardSpec

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
		addPeer := func(peer *network.WireguardPeer) error {
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

			cfg.Peers = append(cfg.Peers, wgtypes.PeerConfig{
				PublicKey:                   pubKey,
				Endpoint:                    endpoint,
				PresharedKey:                presharedKey,
				PersistentKeepaliveInterval: &peer.PersistentKeepaliveInterval,
				ReplaceAllowedIPs:           true,
				AllowedIPs: xslices.Map(peer.AllowedIPs, func(peerIP netip.Prefix) net.IPNet {
					return *netipx.PrefixIPNet(peerIP)
				}),
			})

			return nil
		}

		deletePeer := func(peer *network.WireguardPeer) error {
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

		var left, right *network.WireguardPeer

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
func (a wireguardSpec) Decode(dev *wgtypes.Device, isStatus bool) {
	spec := a.WireguardSpec

	if isStatus {
		spec.PublicKey = dev.PublicKey.String()
	} else {
		spec.PrivateKey = dev.PrivateKey.String()
	}

	spec.ListenPort = dev.ListenPort
	spec.FirewallMark = dev.FirewallMark

	spec.Peers = make([]network.WireguardPeer, len(dev.Peers))

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
		spec.Peers[i].AllowedIPs = xslices.Map(dev.Peers[i].AllowedIPs, func(peerIP net.IPNet) netip.Prefix {
			res, _ := netipx.FromStdIPNet(&peerIP)

			return res
		})
	}
}
