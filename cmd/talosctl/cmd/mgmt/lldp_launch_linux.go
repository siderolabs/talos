// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

func runLLDPAdvertiser(ctx context.Context, bridgeName string) error {
	bridge, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("LLDP bridge: %w", err)
	}

	if bridge.Type() != "bridge" {
		return fmt.Errorf("LLDP interface %q is not a bridge", bridgeName)
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("LLDP packet socket: %w", err)
	}

	defer unix.Close(fd) //nolint:errcheck

	frame := vm.LLDPTestFrame()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err = advertiseLLDP(fd, frame, bridge.Attrs().Index); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func advertiseLLDP(fd int, frame []byte, bridgeIndex int) error {
	// CNI attaches one host veth bridge port per node. Its peer is in the
	// node netns, where tc-redirect-tap redirects ingress to the QEMU TAP.
	// Send directly OUT each host veth, not through the bridge: Linux bridges
	// intentionally do not forward LLDP's 01:80:c2:00:00:0e link-local group.
	// Rediscovery also handles nodes added/restarted after the fixture starts.
	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("LLDP list bridge ports: %w", err)
	}

	for _, link := range links {
		if !lldpBridgePort(link, bridgeIndex) {
			continue
		}

		if sendErr := unix.Sendto(fd, frame, 0, &unix.SockaddrLinklayer{Ifindex: link.Attrs().Index}); sendErr != nil {
			// A node can disappear between discovery and send. Retry on the
			// next tick rather than losing the fixture for all remaining nodes.
			log.Printf("LLDP send on %s: %v", link.Attrs().Name, sendErr)
		}
	}

	return nil
}

func lldpBridgePort(link netlink.Link, bridgeIndex int) bool {
	return link.Type() == "veth" && link.Attrs().MasterIndex == bridgeIndex
}
