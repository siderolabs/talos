// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Additional information can be found
// https://www.kernel.org/doc/Documentation/networking/bonding.txt.

package nic

import (
	"fmt"
	"net"

	"github.com/mdlayher/netx/eui64"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/wglan"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// WithWireguardLanConfig defines the parameters for the Wireguard LAN feature, using auto-discovered Node peers, addressing, routings, and other similar features.
func WithWireguardLanConfig(cfg config.WireguardConfig) Option {
	return func(n *NetworkInterface) (err error) {
		n.WgLanConfig = new(wglan.Config)

		prefix, err := cfg.AutomaticNodesPrefix()
		if err != nil {
			return fmt.Errorf("failed to acquire automatic nodes prefix for %s: %w", n.Name, err)
		}

		autoIP, err := wgEUI64(prefix)
		if err != nil {
			return fmt.Errorf("failed to determine EUI-64 IP address for %q: %w", n.Name, err)
		}

		if n.WireguardConfig.FirewallMark == nil || *n.WireguardConfig.FirewallMark == 0 {
			fwMark := int(constants.WireguardDefaultFirewallMark)

			n.WireguardConfig.FirewallMark = &fwMark
		}

		n.AddressMethod = append(n.AddressMethod, &address.Static{
			CIDR: autoIP.String(),
		})

		n.WgLanConfig = &wglan.Config{
			IP:               autoIP,
			Subnet:           prefix,
			EnablePodRouting: cfg.PodNetworkingEnabled(),
			ForceLocalRoutes: false,           // TODO: not implemented
			ClusterID:        cfg.ClusterID(), // NB: this may be empty, and if so, it will be filled later, when the full machine config is available
			DiscoveryURL:     cfg.NATDiscoveryService(),
		}

		return nil
	}
}

// WithWireguardConfig defines if the interface should be a Wireguard interface and supplies Wireguard configs.
//nolint:gocyclo
func WithWireguardConfig(cfg config.WireguardConfig) Option {
	return func(n *NetworkInterface) (err error) {
		n.Wireguard = true

		privateKey, err := cfg.PrivateKey()
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

func wgEUI64(prefix netaddr.IPPrefix) (out netaddr.IPPrefix, err error) {
	mac, err := firstRealMAC()
	if err != nil {
		return out, fmt.Errorf("failed to find first MAC address: %w", err)
	}

	stdIP, err := eui64.ParseMAC(prefix.IPNet().IP, mac)
	if err != nil {
		return out, fmt.Errorf("failed to parse MAC into EUI-64 address: %w", err)
	}

	ip, ok := netaddr.FromStdIP(stdIP)
	if !ok {
		return out, fmt.Errorf("failed to parse intermediate standard IP %q: %w", stdIP.String(), err)
	}

	return netaddr.IPPrefixFrom(ip, prefix.Bits()), nil
}

func firstRealMAC() (net.HardwareAddr, error) {
	h, err := netlink.NewHandle(0)
	if err != nil {
		return nil, fmt.Errorf("failed to get netlink handle: %w", err)
	}

	list, err := h.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to get list of links: %w", err)
	}

	for _, l := range list {
		if l.Type() == "device" && l.Attrs().Flags&net.FlagLoopback != net.FlagLoopback {
			return l.Attrs().HardwareAddr, nil
		}
	}

	return nil, fmt.Errorf("no physical NICs found")
}
