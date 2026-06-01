// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/siderolabs/gen/xslices"
	sideronet "github.com/siderolabs/net"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

// NewWireguardConfigBundle creates a new Wireguard config bundle.
func NewWireguardConfigBundle(ips []netip.Addr, wireguardCidr string, listenPort, controlplanesCount int) (*WireguardConfigBundle, error) {
	configs := map[netip.Addr]*network.WireguardConfigV1Alpha1{}
	keys := make([]wgtypes.Key, len(ips))
	peers := make([]network.WireguardPeer, len(ips))

	wgCidr, err := netip.ParsePrefix(wireguardCidr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse wireguard cidr %s: %w", wireguardCidr, err)
	}

	for i, ip := range ips {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}

		wgAddr, err := sideronet.NthIPInNetwork(wgCidr, i+2)
		if err != nil {
			return nil, err
		}

		keys[i] = key

		peers[i] = network.WireguardPeer{
			WireguardAllowedIPs: []network.Prefix{
				{
					Prefix: netip.PrefixFrom(wgAddr, wgAddr.BitLen()),
				},
			},
			WireguardPublicKey:                   key.PublicKey().String(),
			WireguardPersistentKeepaliveInterval: time.Second * 5,
		}

		if i < controlplanesCount {
			peers[i].WireguardEndpoint = network.AddrPort{AddrPort: netip.AddrPortFrom(ip, uint16(listenPort))}
		}
	}

	for i, nodeIP := range ips {
		wgAddr, err := sideronet.NthIPInNetwork(wgCidr, i+2)
		if err != nil {
			return nil, err
		}

		config := network.NewWireguardConfigV1Alpha1("wg0")

		config.WireguardPeers = xslices.Filter(peers, func(p network.WireguardPeer) bool {
			return p.WireguardPublicKey != keys[i].PublicKey().String()
		})
		config.WireguardPrivateKey = keys[i].String()
		config.LinkAddresses = []network.AddressConfig{
			{
				AddressAddress: netip.PrefixFrom(wgAddr, wgCidr.Bits()),
			},
		}
		config.LinkUp = new(true)
		config.LinkMTU = 1500

		if i < controlplanesCount {
			config.WireguardListenPort = listenPort
		}

		configs[nodeIP] = config
	}

	return &WireguardConfigBundle{
		configs: configs,
	}, nil
}

// WireguardConfigBundle allows assembling wireguard network configuration with first controlplane being listen node.
type WireguardConfigBundle struct {
	configs map[netip.Addr]*network.WireguardConfigV1Alpha1
}

// PatchNode generates config patch for a node.
func (w *WireguardConfigBundle) PatchNode(ip netip.Addr) (configpatcher.Patch, error) {
	cfg, ok := w.configs[ip]
	if !ok {
		return nil, fmt.Errorf("no wireguard config for ip %s", ip.String())
	}

	ctr, err := container.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create wireguard config container: %w", err)
	}

	return configpatcher.NewStrategicMergePatch(ctr), nil
}
