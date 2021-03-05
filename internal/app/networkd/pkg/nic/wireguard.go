// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Additional information can be found
// https://www.kernel.org/doc/Documentation/networking/bonding.txt.

package nic

import (
	"net"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

// WithWireguardConfig defines if the interface should be a Wireguard interface and supplies Wireguard configs.
//nolint:gocyclo
func WithWireguardConfig(cfg config.WireguardConfig) Option {
	return func(n *NetworkInterface) (err error) {
		n.Wireguard = true

		privateKey, err := wgtypes.ParseKey(cfg.PrivateKey())
		if err != nil {
			return err
		}

		config := &wgtypes.Config{
			PrivateKey:   &privateKey,
			ReplacePeers: true,
		}

		firewallMark := cfg.FirewallMark()
		if firewallMark > 0 {
			config.FirewallMark = &firewallMark
		}

		listenPort := cfg.ListenPort()

		if listenPort > 0 {
			config.ListenPort = &listenPort
		}

		config.Peers = make([]wgtypes.PeerConfig, len(cfg.Peers()))

		for i, peer := range cfg.Peers() {
			publicKey, err := wgtypes.ParseKey(peer.PublicKey())
			if err != nil {
				return err
			}

			peerCfg := wgtypes.PeerConfig{
				PublicKey:  publicKey,
				AllowedIPs: make([]net.IPNet, len(peer.AllowedIPs())),
			}

			if peer.Endpoint() != "" {
				peerCfg.Endpoint, err = net.ResolveUDPAddr("", peer.Endpoint())
				if err != nil {
					return err
				}
			}

			peerKeepaliveInterval := peer.PersistentKeepaliveInterval()

			if peerKeepaliveInterval > 0 {
				peerCfg.PersistentKeepaliveInterval = &peerKeepaliveInterval
			}

			for j, ip := range peer.AllowedIPs() {
				_, ip, err := net.ParseCIDR(ip)
				if err != nil {
					return err
				}

				peerCfg.AllowedIPs[j] = *ip
			}

			config.Peers[i] = peerCfg
		}

		n.WireguardConfig = config

		return nil
	}
}
