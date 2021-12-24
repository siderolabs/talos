// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"net"
	"strings"
	"time"

	talosnet "github.com/talos-systems/net"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// NewWireguardConfigBundle creates a new Wireguard config bundle.
func NewWireguardConfigBundle(ips []net.IP, wireguardCidr string, listenPort, mastersCount int) (*WireguardConfigBundle, error) {
	configs := map[string]*v1alpha1.Device{}
	keys := make([]wgtypes.Key, len(ips))
	peers := make([]*v1alpha1.DeviceWireguardPeer, len(ips))

	for i, ip := range ips {
		key, err := wgtypes.GenerateKey()
		if err != nil {
			return nil, err
		}

		keys[i] = key

		peers[i] = &v1alpha1.DeviceWireguardPeer{
			WireguardAllowedIPs: []string{
				wireguardCidr,
			},
			WireguardPublicKey:                   key.PublicKey().String(),
			WireguardPersistentKeepaliveInterval: time.Second * 5,
		}

		if i < mastersCount {
			peers[i].WireguardEndpoint = fmt.Sprintf("%s:%d", ip.String(), listenPort)
		}
	}

	parts := strings.Split(wireguardCidr, "/")
	networkNumber := parts[1]

	_, network, err := net.ParseCIDR(wireguardCidr)
	if err != nil {
		return nil, err
	}

	for i, nodeIP := range ips {
		wgIP, err := talosnet.NthIPInNetwork(network, i+2)
		if err != nil {
			return nil, err
		}

		config := &v1alpha1.DeviceWireguardConfig{}

		currentPeers := []*v1alpha1.DeviceWireguardPeer{}
		// add all peers except self
		for _, peer := range peers {
			if peer.PublicKey() != keys[i].PublicKey().String() {
				currentPeers = append(currentPeers, peer)
			}
		}

		config.WireguardPeers = currentPeers
		config.WireguardPrivateKey = keys[i].String()

		device := &v1alpha1.Device{
			DeviceInterface:       "wg0",
			DeviceAddresses:       []string{fmt.Sprintf("%s/%s", wgIP.String(), networkNumber)},
			DeviceWireguardConfig: config,
			DeviceMTU:             1500,
		}

		if i < mastersCount {
			config.WireguardListenPort = listenPort
		}

		configs[nodeIP.String()] = device
	}

	return &WireguardConfigBundle{
		configs: configs,
	}, nil
}

// WireguardConfigBundle allows assembling wireguard network configuration with first controlplane being listen node.
type WireguardConfigBundle struct {
	configs map[string]*v1alpha1.Device
}

// PatchConfig generates config patch for a node and patches the configuration data.
func (w *WireguardConfigBundle) PatchConfig(ip fmt.Stringer, cfg config.Provider) (config.Provider, error) {
	config := cfg.Raw().(*v1alpha1.Config).DeepCopy()

	if config.MachineConfig.MachineNetwork == nil {
		config.MachineConfig.MachineNetwork = &v1alpha1.NetworkConfig{
			NetworkInterfaces: []*v1alpha1.Device{},
		}
	}

	device, ok := w.configs[ip.String()]
	if !ok {
		return nil, fmt.Errorf("failed to get wireguard config for node %s", ip.String())
	}

	config.MachineConfig.MachineNetwork.NetworkInterfaces = append(config.MachineConfig.MachineNetwork.NetworkInterfaces, device)

	return config, nil
}
