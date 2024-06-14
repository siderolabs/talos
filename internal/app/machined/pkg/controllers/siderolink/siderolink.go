// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package siderolink provides controllers which manage file resources.
package siderolink

import (
	"fmt"
	"time"

	"github.com/siderolabs/siderolink/pkg/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// WireguardClient allows mocking Wireguard client.
type WireguardClient interface {
	Device(string) (*wgtypes.Device, error)
	Close() error
}

func peerDown(wgClient WireguardClient) (bool, error) {
	wgDevice, err := wgClient.Device(constants.SideroLinkName)
	if err != nil {
		return false, fmt.Errorf("error reading Wireguard device: %w", err)
	}

	if len(wgDevice.Peers) != 1 {
		return false, fmt.Errorf("unexpected number of Wireguard peers: %d", len(wgDevice.Peers))
	}

	peer := wgDevice.Peers[0]
	since := time.Since(peer.LastHandshakeTime)

	return since >= wireguard.PeerDownInterval, nil
}
