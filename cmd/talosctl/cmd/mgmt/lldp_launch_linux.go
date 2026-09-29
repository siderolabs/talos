// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jsimonetti/rtnetlink/v2"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

func runLLDPAdvertiser(ctx context.Context, bridgeName string) error {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("LLDP rtnetlink: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	links, err := conn.Link.List()
	if err != nil {
		return fmt.Errorf("LLDP bridge: %w", err)
	}

	bridgeIndex, err := lldpBridgeIndex(links, bridgeName)
	if err != nil {
		return err
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("LLDP packet socket: %w", err)
	}

	defer unix.Close(fd) //nolint:errcheck

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err = advertiseLLDP(conn, fd, bridgeIndex); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func advertiseLLDP(conn *rtnetlink.Conn, fd int, bridgeIndex uint32) error {
	// CNI attaches one host veth bridge port per node. Its peer is in the
	// node netns, where tc-redirect-tap redirects ingress to the QEMU TAP.
	// Send directly OUT each host veth, not through the bridge: Linux bridges
	// intentionally do not forward LLDP's 01:80:c2:00:00:0e link-local group.
	// Rediscovery also handles nodes added/restarted after the fixture starts.
	links, err := conn.Link.List()
	if err != nil {
		return fmt.Errorf("LLDP list bridge ports: %w", err)
	}

	for _, link := range links {
		if !lldpBridgePort(link, bridgeIndex) {
			continue
		}

		frame, marshalErr := vm.LLDPTestFrame(link.Attributes.Name)
		if marshalErr != nil {
			return fmt.Errorf("LLDP frame on %s: %w", link.Attributes.Name, marshalErr)
		}

		if sendErr := unix.Sendto(fd, frame, 0, &unix.SockaddrLinklayer{Ifindex: int(link.Index)}); sendErr != nil {
			// A node can disappear between discovery and send. Retry on the
			// next tick rather than losing the fixture for all remaining nodes.
			log.Printf("LLDP send on %s: %v", link.Attributes.Name, sendErr)
		}
	}

	return nil
}

func lldpBridgeIndex(links []rtnetlink.LinkMessage, name string) (uint32, error) {
	for _, link := range links {
		if link.Attributes == nil || link.Attributes.Name != name {
			continue
		}

		if link.Attributes.Info == nil || link.Attributes.Info.Kind != "bridge" {
			return 0, fmt.Errorf("LLDP interface %q is not a bridge", name)
		}

		return link.Index, nil
	}

	return 0, fmt.Errorf("LLDP bridge %q not found", name)
}

func lldpBridgePort(link rtnetlink.LinkMessage, bridgeIndex uint32) bool {
	return link.Attributes != nil && link.Attributes.Info != nil &&
		link.Attributes.Info.Kind == "veth" && link.Attributes.Master != nil &&
		*link.Attributes.Master == bridgeIndex
}
