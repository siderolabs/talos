// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lldp

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/mdlayher/packet"
	"golang.org/x/sys/unix"
)

var multicastAddresses = [...]net.HardwareAddr{
	{0x01, 0x80, 0xc2, 0x00, 0x00, 0x0e},
	{0x01, 0x80, 0xc2, 0x00, 0x00, 0x03},
	{0x01, 0x80, 0xc2, 0x00, 0x00, 0x00},
}

type packetListener struct {
	conn   *packet.Conn
	buffer []byte
}

// NewListener opens an LLDP listener on the interface with the given kernel index.
//
// The index identifies one device instance: a link replaced under the same name gets a new index,
// so this fails rather than binding a socket to the wrong device.
func NewListener(linkIndex uint32) (Listener, error) {
	iface, err := net.InterfaceByIndex(int(linkIndex))
	if err != nil {
		return nil, fmt.Errorf("failed to look up link %d for LLDP: %w", linkIndex, err)
	}

	conn, err := packet.Listen(iface, packet.Raw, unix.ETH_P_LLDP, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open LLDP packet socket: %w", err)
	}

	if err = joinMulticastGroups(conn, iface.Index); err != nil {
		conn.Close() //nolint:errcheck

		return nil, err
	}

	return &packetListener{
		conn:   conn,
		buffer: make([]byte, 65536),
	}, nil
}

func joinMulticastGroups(conn *packet.Conn, ifIndex int) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("failed to access LLDP packet socket: %w", err)
	}

	var membershipErr error

	if err = rawConn.Control(func(fd uintptr) {
		for _, address := range multicastAddresses {
			request := unix.PacketMreq{
				Ifindex: int32(ifIndex),
				Type:    unix.PACKET_MR_MULTICAST,
				Alen:    uint16(len(address)),
			}
			copy(request.Address[:], address)

			if membershipErr = unix.SetsockoptPacketMreq(int(fd), unix.SOL_PACKET, unix.PACKET_ADD_MEMBERSHIP, &request); membershipErr != nil {
				return
			}
		}
	}); err != nil {
		return fmt.Errorf("failed to control LLDP packet socket: %w", err)
	}

	if membershipErr != nil {
		return fmt.Errorf("failed to join LLDP multicast group: %w", membershipErr)
	}

	return nil
}

func (listener *packetListener) SetReadDeadline(deadline time.Time) error {
	return listener.conn.SetReadDeadline(deadline)
}

func (listener *packetListener) ReadFrame() ([]byte, error) {
	n, _, err := listener.conn.ReadFrom(listener.buffer)
	if err != nil {
		return nil, err
	}

	return listener.buffer[:n], nil
}

func (listener *packetListener) Close() error {
	err := listener.conn.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}

	return err
}
